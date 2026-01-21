package interfaces

import (
	"context"
	"time"

	"github.com/vultisig/pluginagent/types"
)

// DatabaseStorage defines the interface for database storage operations
type DatabaseStorage interface {
	Close() error

	InsertEvent(ctx context.Context, event *types.SystemEvent) (int64, error)
	GetEventsAfterTimestamp(ctx context.Context, createdAt time.Time) ([]types.SystemEvent, error)

	// Transaction support
	WithTx(ctx context.Context, fn func(DatabaseStorage) error) error
}
