package config

import (
	"github.com/spf13/viper"

	plugin_config "github.com/vultisig/verifier/plugin/config"
	"github.com/vultisig/verifier/plugin/server"
	"github.com/vultisig/verifier/vault_config"

	"github.com/vultisig/pluginagent/logging"
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
	cfg := &Config{}

	viper.SetConfigName("agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func LoadWorkerConfig() (*Config, error) {
	cfg := &Config{}

	viper.SetConfigName("agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
