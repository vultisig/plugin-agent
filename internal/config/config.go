package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"

	plugin_config "github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/plugin/server"
	"github.com/vultisig/verifier/vault_config"

	"github.com/vultisig/pluginagent/internal/logging"
)

type Config struct {
	Redis        plugin_config.Redis       `mapstructure:"redis" json:"redis"`
	VaultService vault_config.Config       `mapstructure:"vault_service" json:"vault_service,omitempty"`
	BlockStorage vault_config.BlockStorage `mapstructure:"block_storage" json:"block_storage,omitempty"`
	Server       server.Config             `mapstructure:"server" json:"server,omitempty"`
	Database     plugin_config.Database    `mapstructure:"database" json:"database,omitempty"`
	Plugin       PluginConfig              `mapstructure:"plugin" json:"plugin,omitempty"`
	Verifier     plugin_config.Verifier    `mapstructure:"verifier" json:"verifier,omitempty"`
	LogFormat    logging.LogFormat         `mapstructure:"log_format" json:"log_format,omitempty"`
	HealthPort   int                       `mapstructure:"health_port" json:"health_port,omitempty"`
}

type PluginConfig struct {
	PluginID     string `mapstructure:"plugin_id" json:"plugin_id,omitempty"`
	SpecFilePath string `mapstructure:"file_path" json:"file_path,omitempty"`
}

func LoadServerConfig() (*Config, error) {
	configName := os.Getenv("VS_SERVER_CONFIG_NAME")
	if configName == "" {
		configName = "config"
	}

	return ReadConfig(configName)
}

func ReadConfig(configName string) (*Config, error) {
	viper.SetConfigName(configName)
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("Server.VaultsFilePath", "vaults")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file, %w", err)
	}
	var cfg Config
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %w", err)
	}
	return &cfg, nil
}

func LoadWorkerConfig() (*Config, error) {
	configName := os.Getenv("VS_WORKER_CONFIG_NAME")
	if configName == "" {
		configName = "config"
	}

	return ReadConfig(configName)
}
