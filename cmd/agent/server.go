package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/vultisig/pluginagent/internal/api"
	"github.com/vultisig/pluginagent/internal/policy"
	"github.com/vultisig/verifier/plugin"
	vpolicy "github.com/vultisig/verifier/plugin/policy"
	"github.com/vultisig/verifier/plugin/policy/policy_pg"
	"github.com/vultisig/verifier/plugin/redis"
	"github.com/vultisig/verifier/plugin/scheduler"

	"github.com/vultisig/verifier/vault"
	vgrelay "github.com/vultisig/vultisig-go/relay"

	"github.com/vultisig/pluginagent/internal/config"
	"github.com/vultisig/pluginagent/internal/storage"
)

func main() {
	cfg, err := config.LoadServerConfig()
	if err != nil {
		panic(err)
	}
	logger := logrus.New()

	redisClient, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		logger.Fatalf("failed to initialize Redis client: %v", err)
	}

	asynqConnOpt, err := asynq.ParseRedisURI(cfg.Redis.URI)
	if err != nil {
		logger.Fatalf("failed to parse redis URI: %v", err)
	}

	client := asynq.NewClient(asynqConnOpt)
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Println("fail to close asynq client,", err)
		}
	}()

	inspector := asynq.NewInspector(asynqConnOpt)

	vaultStorage, err := vault.NewBlockStorageImp(cfg.BlockStorage)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgPool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		logger.Fatalf("failed to initialize Postgres pool: %v", err)
	}

	policyStorage, err := plugin.WithMigrations(
		logger,
		pgPool,
		policy_pg.NewRepo,
		"policy/policy_pg/migrations",
	)
	if err != nil {
		logger.Fatalf("failed to initialize policy storage: %v", err)
	}

	policyService, err := vpolicy.NewPolicyService(
		policyStorage,
		scheduler.NewNilService(),
		logger,
	)
	if err != nil {
		logger.Fatalf("failed to initialize policy service: %v", err)
	}

	db, err := storage.NewDatabaseStorage(logger, storage.StorageConfig{
		Type: storage.StorageTypePostgreSQL,
		Pool: pgPool,
	})
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	relayClient := vgrelay.NewRelayClient(cfg.VaultService.Relay.Server)

	spec, err := policy.NewAgentSpec(cfg.Plugin.SpecFilePath)
	if err != nil {
		logger.Fatalf("failed to initialize agent spec: %v", err)
	}

	server := api.NewServer(
		cfg.Server,
		cfg.Plugin,
		db,
		policyService,
		redisClient,
		vaultStorage,
		client,
		inspector,
		relayClient,
		cfg.Verifier,
		spec,
	)

	if err := server.Start(ctx); err != nil {
		panic(err)
	}
}
