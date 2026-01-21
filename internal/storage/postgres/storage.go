package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/policy/policy_pg"

	"github.com/vultisig/pluginagent/internal/storage/interfaces"
	"github.com/vultisig/pluginagent/internal/storage/postgres/queries"
	"github.com/vultisig/pluginagent/internal/types"
)

var _ interfaces.DatabaseStorage = (*Storage)(nil)

type MigrationOptions struct {
	RunPluginMigrations bool
}

type Storage struct {
	pool    *pgxpool.Pool
	queries *queries.Queries
	*policy_pg.Repo
}

func NewPostgresStorage(logger *logrus.Logger, pgPool *pgxpool.Pool) (interfaces.DatabaseStorage, error) {
	return NewPostgresStorageWithOptions(logger, pgPool, &MigrationOptions{
		RunPluginMigrations: true,
	})
}

func NewPostgresStorageWithOptions(logger *logrus.Logger, pgPool *pgxpool.Pool, opts *MigrationOptions) (interfaces.DatabaseStorage, error) {
	policyStorage, err := plugin.WithMigrations(
		logger,
		pgPool,
		policy_pg.NewRepo,
		"policy/policy_pg/migrations",
	)
	if err != nil {
		return nil, fmt.Errorf("error creating policy storage: %w", err)
	}

	backend := &Storage{pgPool, queries.New(pgPool), policyStorage}

	if opts == nil {
		opts = &MigrationOptions{
			RunPluginMigrations: true,
		}
	}

	if err := backend.migrate(opts); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return backend, nil
}

func (s *Storage) Close() error {
	s.pool.Close()
	return nil
}

func (s *Storage) migrate(opts *MigrationOptions) error {
	logrus.Info("Starting database migration...")

	if opts.RunPluginMigrations {
		pluginMgr := NewPluginMigrationManager(s.pool)
		if err := pluginMgr.Migrate(); err != nil {
			return fmt.Errorf("failed to run plugin migrations: %w", err)
		}
	}

	logrus.Info("Database migration completed successfully")
	return nil
}

func (s *Storage) WithTx(ctx context.Context, fn func(interfaces.DatabaseStorage) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			logrus.WithError(err).Error("failed to rollback transaction")
		}
	}()

	txStorage := &Storage{s.pool, s.queries.WithTx(tx), s.Repo}

	if err := fn(txStorage); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) InsertEvent(ctx context.Context, event *types.SystemEvent) (int64, error) {
	var policyID pgtype.UUID
	if event.PolicyID != nil {
		policyID = uuidToPgUUID(*event.PolicyID)
	}

	params := queries.InsertEventParams{
		PublicKey: pgtype.Text{String: *event.PublicKey, Valid: event.PublicKey != nil},
		PolicyID:  policyID,
		EventType: queries.SystemEventType(event.EventType),
		EventData: event.EventData,
	}

	return s.queries.InsertEvent(ctx, params)
}

func (s *Storage) GetEventsAfterTimestamp(ctx context.Context, createdAt time.Time) ([]types.SystemEvent, error) {
	rows, err := s.queries.GetEventsAfterTimestamp(ctx, pgtype.Timestamp{Time: createdAt, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	events := make([]types.SystemEvent, 0, len(rows))
	for _, row := range rows {
		event, err := toTypesSystemEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, nil
}
