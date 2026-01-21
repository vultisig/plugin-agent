package types

import (
	"time"

	"github.com/google/uuid"
)

type SystemEventType string

const (
	SystemEventTypeVaultReshared       SystemEventType = "vault_reshared"
	SystemEventTypeVaultDeleted        SystemEventType = "vault_deleted"
	SystemEventTypePluginPolicyCreated SystemEventType = "policy_created"
	SystemEventTypePluginPolicyDeleted SystemEventType = "policy_deleted"
)

type SystemEvent struct {
	ID        int64
	PublicKey *string
	PolicyID  *uuid.UUID
	EventType SystemEventType
	EventData []byte
	CreatedAt time.Time
}
