package config

import (
	"fmt"
	"merionyx/api-gateway/internal/shared/etcd"

	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Etcd         etcd.EtcdConfig    `mapstructure:"etcd"`
	Repositories []RepositoryConfig `mapstructure:"repositories"`
	APIServer    APIServerConfig    `mapstructure:"api_server"`
}

type ServerConfig struct {
	GRPCPort string `mapstructure:"grpc_port"`
	Host     string `mapstructure:"host"`
}

type APIServerConfig struct {
	Address string `mapstructure:"address"`
}

type RepositoryConfig struct {
	Name   string         `mapstructure:"name"`
	Source string         `mapstructure:"source"`
	URL    string         `mapstructure:"url"`
	Path   string         `mapstructure:"path"`
	Auth   AuthConfig     `mapstructure:"auth"`
}

type AuthConfig struct {
	Type     string `mapstructure:"type"`
	SSHKeyPath   string `mapstructure:"ssh_key_path"`
	SSHKeyEnv    string `mapstructure:"ssh_key_env"`
	TokenEnv     string `mapstructure:"token_env"`
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "./configs/contract-syncer/config.yaml"
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
