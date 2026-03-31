package config

import (
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/shared/etcd"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig        `mapstructure:"server" validate:"required" json:"server"`
	Etcd         etcd.EtcdConfig     `mapstructure:"etcd"`
	Repositories []RepositoryConfig  `mapstructure:"repositories" validate:"required" json:"repositories"`
	Environments []EnvironmentConfig `mapstructure:"environments" validate:"required" json:"environments"`
	Services     ServicesConfig      `mapstructure:"services" validate:"required" json:"services"`
}

type ServerConfig struct {
	HTTP1Port string `mapstructure:"http1_port" validate:"required" json:"http1_port"`
	HTTP2Port string `mapstructure:"http2_port" validate:"required" json:"http2_port"`
	GRPCPort  string `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	XDSPort   string `mapstructure:"xds_port" validate:"required" json:"xds_port"`
	Host      string `mapstructure:"host" json:"host"`
}

type RepositoryConfig struct {
	Name   string     `mapstructure:"name" validate:"required" json:"name"`
	Source string     `mapstructure:"source" validate:"required" json:"source"` // "git", "local-git", "local-dir"
	URL    string     `mapstructure:"url" json:"url"`                           // для source: git
	Path   string     `mapstructure:"path" json:"path"`                         // для source: local-git, local-dir
	Auth   AuthConfig `mapstructure:"auth" json:"auth"`
}

type AuthConfig struct {
	Type       string `mapstructure:"type" json:"type"` // "ssh", "token", "none"
	SSHKeyPath string `mapstructure:"ssh_key_path" json:"ssh_key_path"`
	SSHKeyEnv  string `mapstructure:"ssh_key_env" json:"ssh_key_env"`
	TokenEnv   string `mapstructure:"token_env" json:"token_env"`
}

type EnvironmentConfig struct {
	Name     string         `mapstructure:"name" validate:"required" json:"name"`
	Bundles  BundlesConfig  `mapstructure:"bundles" validate:"required" json:"bundles"`
	Services ServicesConfig `mapstructure:"services" validate:"required" json:"services"`
}

type BundlesConfig struct {
	Static []StaticBundleConfig `mapstructure:"static" validate:"required" json:"static"`
}

type StaticBundleConfig struct {
	Name       string `mapstructure:"name" validate:"required" json:"name"`
	Repository string `mapstructure:"repository" validate:"required" json:"repository"`
	Ref        string `mapstructure:"ref" validate:"required" json:"ref"`
	Path       string `mapstructure:"path" validate:"required" json:"path"`
}

type ServicesConfig struct {
	Static []StaticServiceConfig `mapstructure:"static" validate:"required" json:"static"`
}

type StaticServiceConfig struct {
	Name     string `mapstructure:"name" validate:"required" json:"name"`
	Upstream string `mapstructure:"upstream" validate:"required" json:"upstream"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Set default values
	viper.SetDefault("server.http1_port", "8080")
	viper.SetDefault("server.http2_port", "8443")
	viper.SetDefault("server.grpc_port", "19090")
	viper.SetDefault("server.xds_port", "19091")
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("logging.enabled", false)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "[${time}] ${status} - ${method} ${path}")
	viper.SetDefault("repositories.auth.type", "none")

	// Support environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("CP_")

	// If a specific config file is passed
	if len(configFile) > 0 && configFile[0] != "" {
		slog.Info(fmt.Sprintf("Loading config from %s", configFile[0]))
		viper.SetConfigFile(configFile[0])
	} else {
		// Default settings for finding the file
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")                       // Current directory
		viper.AddConfigPath("./config")                // Subdirectory config
		viper.AddConfigPath("./configs/control-plane") // Subdirectory configs/app
	}

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Info("Config file not found, using defaults and environment variables")
		} else {
			slog.Error(fmt.Sprintf("Error reading config file: %v", err))
			return nil, err
		}
	} else {
		slog.Info(fmt.Sprintf("Using config file %s", viper.ConfigFileUsed()))
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Check if all repositories are valid
	for _, repository := range config.Repositories {
		switch repository.Source {
		case "git":
			if repository.URL == "" {
				return nil, fmt.Errorf("url is required for git repository")
			}

			if repository.Auth.Type == "ssh" {
				if repository.Auth.SSHKeyPath == "" && repository.Auth.SSHKeyEnv == "" {
					return nil, fmt.Errorf("ssh_key_path or ssh_key_env is required for ssh authentication")
				}

				// Check if the ssh key path is valid
				if repository.Auth.SSHKeyPath != "" {
					if _, err := os.Stat(repository.Auth.SSHKeyPath); err != nil {
						return nil, fmt.Errorf("ssh_key_path is not valid: %v", err)
					}
				} else if repository.Auth.SSHKeyEnv != "" {
					if os.Getenv(repository.Auth.SSHKeyEnv) == "" {
						return nil, fmt.Errorf("ssh_key_env is not set: %s", repository.Auth.SSHKeyEnv)
					}
				} else {
					return nil, fmt.Errorf("ssh_key_path or ssh_key_env is required for ssh authentication")
				}
			}

			if repository.Auth.Type == "token" {
				if repository.Auth.TokenEnv == "" {
					return nil, fmt.Errorf("token_env is required for token authentication")
				}

				// Check if the token env is valid
				if os.Getenv(repository.Auth.TokenEnv) == "" {
					return nil, fmt.Errorf("token_env is not set: %s", repository.Auth.TokenEnv)
				}
			}
		case "local-git":
			if repository.Path == "" {
				return nil, fmt.Errorf("path is required for local-git repository")
			}

			if _, err := os.Stat(repository.Path); err != nil {
				return nil, fmt.Errorf("path is not valid: %v", err)
			}
		case "local-dir":
			if repository.Path == "" {
				return nil, fmt.Errorf("path is required for local-dir repository")
			}

			if _, err := os.Stat(repository.Path); err != nil {
				return nil, fmt.Errorf("path is not valid: %v", err)
			}
		default:
			return nil, fmt.Errorf("unsupported source %s for repository", repository.Source)
		}
	}

	return &config, nil
}
