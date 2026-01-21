package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"github.com/vultisig/pluginagent/internal/types"
)

type WebSocketMessage struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type SubscriptionRequest struct {
	Channel  string `json:"channel"`
	LastSeen *int64 `json:"last_seen,omitempty"`
}

type EventMessage struct {
	ID        int64                 `json:"id"`
	PublicKey *string               `json:"public_key"`
	PolicyID  *string               `json:"policy_id,omitempty"`
	EventType types.SystemEventType `json:"event_type"`
	EventData json.RawMessage       `json:"event_data"`
	CreatedAt time.Time             `json:"created_at"`
}

type ClientConnection struct {
	ws                 *websocket.Conn
	subscriptions      map[string]bool
	mutex              sync.RWMutex
	replayMutex        sync.Mutex
	isStreamingHistory bool
	lastHistoryEventID int64
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients             = make(map[*ClientConnection]bool)
	clientsMutex        = sync.RWMutex{}
	lastStreamedEventID int64
	streamEventsMutex   = sync.RWMutex{}
)

func (s *Server) GetEvents(c echo.Context) error {
	s.logger.Info("GetEvents WebSocket upgrade")

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.WithError(err).Error("WebSocket upgrade failed")
		return err
	}

	s.logger.Info("WebSocket connected")
	defer func() {
		err = ws.Close()
		if err != nil {
			s.logger.WithError(err).Error("WebSocket close failed")
		}
		s.logger.Info("WebSocket disconnected")
	}()

	client := &ClientConnection{
		ws:            ws,
		subscriptions: make(map[string]bool),
	}

	clientsMutex.Lock()
	clients[client] = true
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(clients, client)
		clientsMutex.Unlock()
	}()

	for {
		var msg WebSocketMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			s.logger.WithError(err).Debug("WebSocket receive error")
			return nil
		}

		switch msg.Type {
		case "subscribe":
			var subReq SubscriptionRequest
			data, _ := json.Marshal(msg.Data)
			if err := json.Unmarshal(data, &subReq); err != nil {
				s.sendError(ws, "invalid subscription request")
				continue
			}

			if subReq.Channel == "system_events" {
				s.handleSystemEventsSubscription(client, subReq, c)
			} else {
				s.sendError(ws, "unknown channel")
			}

		default:
			s.sendError(ws, "unknown message type")
		}
	}
}

func (s *Server) handleSystemEventsSubscription(client *ClientConnection, req SubscriptionRequest, ctx echo.Context) {
	client.mutex.Lock()
	client.subscriptions["system_events"] = true
	client.mutex.Unlock()

	go func() {
		err := s.sendMessage(client.ws, WebSocketMessage{
			Type: "subscription_confirmed",
			Data: map[string]string{"channel": "system_events"},
		})
		if err != nil {
			s.logger.WithError(err).Error("WebSocket send error")
		}
		client.replayMutex.Lock()
		defer client.replayMutex.Unlock()

		client.mutex.Lock()
		client.isStreamingHistory = true
		client.mutex.Unlock()

		var lastHistoryID int64
		var lastEventTime time.Time

		if req.LastSeen != nil {
			lastSeenTime := time.UnixMilli(*req.LastSeen)
			events, err := s.db.GetEventsAfterTimestamp(ctx.Request().Context(), lastSeenTime)
			if err != nil {
				s.logger.WithError(err).Error("Failed to get historical events")
				s.sendError(client.ws, "failed to get historical events")
				client.mutex.Lock()
				client.isStreamingHistory = false
				client.mutex.Unlock()
				return
			}

			for _, event := range events {
				eventMsg := s.convertToEventMessage(event)
				if err := s.sendEvent(client.ws, eventMsg); err != nil {
					s.logger.WithError(err).Debug("Failed to send historical event")
					client.mutex.Lock()
					client.isStreamingHistory = false
					client.mutex.Unlock()
					return
				}
				lastHistoryID = event.ID
			}

			if len(events) > 0 {
				lastEventTime = events[len(events)-1].CreatedAt
			}
		}

		client.mutex.Lock()
		client.isStreamingHistory = false
		client.lastHistoryEventID = lastHistoryID
		client.mutex.Unlock()

		if lastHistoryID > 0 {
			streamEventsMutex.RLock()
			currentStreamedID := lastStreamedEventID
			streamEventsMutex.RUnlock()

			if currentStreamedID > lastHistoryID {
				gapStartTime := lastEventTime.Add(1 * time.Nanosecond)
				if lastEventTime.IsZero() {
					gapStartTime = time.Unix(*req.LastSeen, 0)
				}

				gapEvents, err := s.db.GetEventsAfterTimestamp(ctx.Request().Context(), gapStartTime)
				if err != nil {
					s.logger.WithError(err).Error("Failed to get gap events")
				} else {
					for _, event := range gapEvents {
						if event.ID > lastHistoryID && event.ID <= currentStreamedID {
							eventMsg := s.convertToEventMessage(event)
							if err := s.sendEvent(client.ws, eventMsg); err != nil {
								s.logger.WithError(err).Debug("Failed to send gap event")
								return
							}
						}
					}
				}
			}
		}
	}()
}

func (s *Server) convertToEventMessage(event types.SystemEvent) EventMessage {
	var policyIDStr *string
	if event.PolicyID != nil {
		policyStr := event.PolicyID.String()
		policyIDStr = &policyStr
	}

	return EventMessage{
		ID:        event.ID,
		PublicKey: event.PublicKey,
		PolicyID:  policyIDStr,
		EventType: event.EventType,
		EventData: event.EventData,
		CreatedAt: event.CreatedAt,
	}
}

func (s *Server) sendEvent(ws *websocket.Conn, event EventMessage) error {
	return s.sendMessage(ws, WebSocketMessage{
		Type: "event",
		Data: event,
	})
}

func (s *Server) sendMessage(ws *websocket.Conn, msg WebSocketMessage) error {
	return ws.WriteJSON(msg)
}

func (s *Server) sendError(ws *websocket.Conn, errorMsg string) {
	err := s.sendMessage(ws, WebSocketMessage{
		Type: "error",
		Data: map[string]string{"message": errorMsg},
	})
	if err != nil {
		s.logger.WithError(err).Error("WebSocket send error")
	}
}

func (s *Server) streamNewEvents() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastSeen := time.Now().UTC()

	s.logger.Info("Starting event streamer")

	for range ticker.C {
		events, err := s.db.GetEventsAfterTimestamp(context.Background(), lastSeen)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get new events")
			continue
		}

		if len(events) == 0 {
			s.logger.WithField("last_seen", lastSeen).Info("No new events found")
			continue
		}

		s.logger.WithField("events", len(events)).Info("Streaming new events")

		clientsMutex.RLock()
		activeClients := make([]*ClientConnection, 0, len(clients))
		for client := range clients {
			client.mutex.RLock()
			if client.subscriptions["system_events"] && !client.isStreamingHistory {
				activeClients = append(activeClients, client)
			}
			client.mutex.RUnlock()
		}
		clientsMutex.RUnlock()

		s.logger.WithField("active_clients", len(activeClients)).Info("Found active clients")

		for _, event := range events {
			eventMsg := s.convertToEventMessage(event)

			for _, client := range activeClients {
				if err := s.sendEvent(client.ws, eventMsg); err != nil {
					s.logger.WithError(err).Debug("Failed to send new event to client")
				}
			}

			streamEventsMutex.Lock()
			if event.ID > lastStreamedEventID {
				lastStreamedEventID = event.ID
			}
			streamEventsMutex.Unlock()

			lastSeen = event.CreatedAt.Add(1 * time.Millisecond)
		}
	}
}
