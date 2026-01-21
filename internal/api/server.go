package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/vultisig/verifier/plugin"
	plugin_config "github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/plugin/keysign"
	"github.com/vultisig/verifier/plugin/policy"
	"github.com/vultisig/verifier/plugin/redis"
	"github.com/vultisig/verifier/plugin/server"
	"github.com/vultisig/verifier/plugin/tasks"
	"github.com/vultisig/verifier/vault"
	vgrelay "github.com/vultisig/vultisig-go/relay"

	"github.com/vultisig/pluginagent/internal/config"
	"github.com/vultisig/pluginagent/internal/storage/interfaces"
)

type Server struct {
	cfg           server.Config
	db            interfaces.DatabaseStorage
	redis         *redis.Redis
	vaultStorage  vault.Storage
	client        *asynq.Client
	policyService policy.Service
	inspector     *asynq.Inspector
	logger        *logrus.Logger
	signer        *keysign.Signer
	pluginCfg     config.PluginConfig
	spec          plugin.Spec
}

// NewServer returns a new server.
func NewServer(
	cfg server.Config,
	pluginCfg config.PluginConfig,
	db interfaces.DatabaseStorage,
	policy policy.Service,
	redis *redis.Redis,
	vaultStorage vault.Storage,
	client *asynq.Client,
	inspector *asynq.Inspector,
	relayClient *vgrelay.Client,
	verifierCfg plugin_config.Verifier,
	spec plugin.Spec,
) *Server {
	logger := logrus.WithField("service", "plugin").Logger

	signer := keysign.NewSigner(
		logger.WithField("pkg", "keysign.Signer").Logger,
		relayClient,
		[]keysign.Emitter{
			keysign.NewVerifierEmitter(verifierCfg.URL, verifierCfg.Token),
			keysign.NewPluginEmitter(client, tasks.TypeKeySignDKLS, tasks.QUEUE_NAME),
		},
		[]string{
			verifierCfg.PartyPrefix,
			"vultisig-tester-ae1d",
		},
	)

	return &Server{
		cfg:           cfg,
		pluginCfg:     pluginCfg,
		redis:         redis,
		client:        client,
		inspector:     inspector,
		vaultStorage:  vaultStorage,
		db:            db,
		logger:        logger,
		policyService: policy,
		signer:        signer,
		spec:          spec,
	}
}

func (s *Server) Start(ctx context.Context) error {
	limiterStore := middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: 5, Burst: 30, ExpiresIn: 5 * time.Minute},
	)
	middlewares := []echo.MiddlewareFunc{
		middleware.Recover(),
		middleware.Logger(),
		middleware.CORS(),
		middleware.BodyLimit("2M"),
		middleware.RateLimiter(limiterStore),
	}
	e := server.NewServer(
		s.cfg,
		s.policyService,
		s.redis,
		s.vaultStorage,
		s.client,
		s.inspector,
		s.spec,
		middlewares,
		nil, // safe to use
		s.logger,
		nil, // safe to use
	).GetRouter()

	// Define specific api endpoints for agent
	e.GET("/ping", s.Ping)
	e.GET("/events", s.GetEvents)
	e.GET("/address/derive", s.DeriveAddress)

	e.POST("/propose", s.Propose)

	go s.streamNewEvents()

	eg := &errgroup.Group{}
	eg.Go(func() error {
		err := e.Start(fmt.Sprintf(":%d", s.cfg.Port))
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("failed to start server: %w", err)
	})
	eg.Go(func() error {
		<-ctx.Done()
		s.logger.Info("shutting down server...")

		c, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		err := e.Shutdown(c)
		if err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
		return nil
	})

	return eg.Wait()
}

func (s *Server) Ping(c echo.Context) error {
	return c.String(http.StatusOK, "Plugin agent is running")
}
