package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Repositories []RepositoryConfig `mapstructure:"repositories"`
}

type ServerConfig struct {
	HTTP1Port string `mapstructure:"http1_port" validate:"required"`
	HTTP2Port string `mapstructure:"http2_port" validate:"required"`
	GRPCPort  string `mapstructure:"grpc_port" validate:"required"`
	Host      string `mapstructure:"host"`
}

type RepositoryConfig struct {
	Name string     `mapstructure:"name" validate:"required"`
	URL  string     `mapstructure:"url" validate:"required"`
	Auth AuthConfig `mapstructure:"auth"`
}
type AuthConfig struct {
	Type       string `mapstructure:"type"` // "ssh", "token", "none"
	SSHKeyPath string `mapstructure:"ssh_key_path"`
	SSHKeyEnv  string `mapstructure:"ssh_key_env"`
	TokenEnv   string `mapstructure:"token_env"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Set default values
	viper.SetDefault("server.http1_port", "8080")
	viper.SetDefault("server.http2_port", "8443")
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

	return &config, nil
}
