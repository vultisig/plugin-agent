package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/plugin/tasks"
	"github.com/vultisig/verifier/plugin/tx_indexer"
	tx_storage "github.com/vultisig/verifier/plugin/tx_indexer/pkg/storage"
	vtypes "github.com/vultisig/verifier/types"
	"github.com/vultisig/verifier/vault"

	"github.com/vultisig/pluginagent/internal/config"
	"github.com/vultisig/pluginagent/internal/health"
	"github.com/vultisig/pluginagent/internal/logging"
	"github.com/vultisig/pluginagent/internal/storage"
	"github.com/vultisig/pluginagent/internal/storage/interfaces"
	"github.com/vultisig/pluginagent/types"
)

func main() {
	cfg, err := config.LoadWorkerConfig()
	if err != nil {
		panic(err)
	}
	logger := logging.NewLogger(cfg.LogFormat)

	vaultStorage, err := vault.NewBlockStorageImp(cfg.BlockStorage)
	if err != nil {
		logger.Fatalf("failed to initialize vault storage: %v", err)
	}

	redisConnOpt, err := asynq.ParseRedisURI(cfg.Redis.URI)
	if err != nil {
		logger.Fatalf("failed to parse redis URI: %v", err)
	}

	client := asynq.NewClient(redisConnOpt)
	consumer := asynq.NewServer(
		redisConnOpt,
		asynq.Config{
			Logger:      logger,
			Concurrency: 10,
			Queues: map[string]int{
				tasks.QUEUE_NAME: 10,
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	pgPool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		logger.Fatalf("failed to initialize Postgres pool: %v", err)
	}

	db, err := storage.NewDatabaseStorage(logger, storage.StorageConfig{
		Type: storage.StorageTypePostgreSQL,
		Pool: pgPool,
	})
	if err != nil {
		logger.Fatalf("failed to initialize redis storage: %v", err)
	}

	txStorage, err := plugin.WithMigrations(
		logger,
		pgPool,
		tx_storage.NewRepo,
		"tx_indexer/pkg/storage/migrations",
	)
	if err != nil {
		logger.Fatalf("failed to initialize Postgres pool: %v", err)
	}

	supportedChains, err := tx_indexer.Chains()
	if err != nil {
		logger.Fatalf("failed to get supported chains: %v", err)
	}

	txIndexerService := tx_indexer.NewService(
		logger,
		txStorage,
		supportedChains,
	)

	vaultMgmService, err := vault.NewManagementService(
		cfg.VaultService,
		client,
		vaultStorage,
		txIndexerService,
		nil,
	)

	healthServer := health.New(cfg.HealthPort)
	go func() {
		er := healthServer.Start(ctx, logger)
		if er != nil {
			logger.Errorf("health server failed: %v", er)
		}
	}()

	if err != nil {
		logger.Fatalf("failed to initialize vault management service: %v", err)
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeKeyGenerationDKLS, resultWriter(db, vaultMgmService.HandleKeyGenerationDKLS))
	mux.HandleFunc(tasks.TypeKeySignDKLS, resultWriter(db, vaultMgmService.HandleKeySignDKLS))
	mux.HandleFunc(tasks.TypeReshareDKLS, resultWriter(db, vaultMgmService.HandleReshareDKLS))

	err = consumer.Run(mux)
	if err != nil {
		logger.Fatalf("failed to run consumer: %v", err)
	}
}

func resultWriter(db interfaces.DatabaseStorage, handler asynq.HandlerFunc) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		err := handler(ctx, task)
		if err != nil {
			return err
		}

		switch task.Type() {
		case tasks.TypeReshareDKLS:
			var taskData vtypes.ReshareRequest
			if err := json.Unmarshal(task.Payload(), &taskData); err != nil {
				return err
			}

			// Record vault resharing event
			event := &types.SystemEvent{
				PublicKey: &taskData.PublicKey,
				PolicyID:  nil,
				EventType: types.SystemEventTypeVaultReshared,
				EventData: task.Payload(),
			}
			_, err = db.InsertEvent(ctx, event)
			if err != nil {
				fmt.Printf("failed to insert event: %v", err)
				return nil
			}
		}

		return nil
	}
}
