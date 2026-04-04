package config

import (
	"fmt"
	"os"
	"strings"
	"time"

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

	// Isolated instance: global viper can be mutated by other imports; Unmarshal must see the same tree as Set.
	v := viper.New()
	v.SetDefault("server.grpc_port", "19092")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("etcd.dial_timeout", "5s")

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyContractSyncerDefaults(&config)

	return &config, nil
}

func applyContractSyncerDefaults(c *Config) {
	if strings.TrimSpace(c.Server.GRPCPort) == "" {
		c.Server.GRPCPort = "19092"
	}
	if strings.TrimSpace(c.Server.Host) == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Etcd.DialTimeout <= 0 {
		c.Etcd.DialTimeout = 5 * time.Second
	}
	if strings.TrimSpace(c.APIServer.Address) == "" {
		if a := strings.TrimSpace(os.Getenv("API_SERVER_ADDRESS")); a != "" {
			c.APIServer.Address = a
		}
	}
	if !c.Etcd.TLS.Enabled {
		for _, ep := range c.Etcd.Endpoints {
			if strings.HasPrefix(strings.TrimSpace(ep), "https://") {
				c.Etcd.TLS = etcd.EtcdTLSConfig{
					Enabled:  true,
					CertFile: "/etc/etcd-tls/tls.crt",
					KeyFile:  "/etc/etcd-tls/tls.key",
					CAFile:   "/etc/etcd-tls/ca.crt",
				}
				break
			}
		}
	}
}
