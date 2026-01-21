package storage

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/vultisig/pluginagent/storage/interfaces"
	"github.com/vultisig/pluginagent/storage/postgres"
)

type StorageType string

const (
	StorageTypePostgreSQL StorageType = "postgresql"
	StorageTypeSQLite     StorageType = "sqlite"
)

type StorageConfig struct {
	Type StorageType
	Pool *pgxpool.Pool
}

// NewDatabaseStorage creates a new database storage instance based on the config.
func NewDatabaseStorage(logger *logrus.Logger, config StorageConfig) (interfaces.DatabaseStorage, error) {
	switch config.Type {
	case StorageTypePostgreSQL:
		return postgres.NewPostgresStorage(logger, config.Pool)
	case StorageTypeSQLite:
		return nil, fmt.Errorf("sqlite storage not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}
