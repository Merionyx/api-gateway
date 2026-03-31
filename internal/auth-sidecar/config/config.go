package config

import (
	"fmt"
	"log/slog"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server" validate:"required" json:"server"`
	Controller ControllerConfig `mapstructure:"controller" validate:"required" json:"controller"`
	JWT        JWTConfig        `mapstructure:"jwt" validate:"required" json:"jwt"`
}

type ServerConfig struct {
	GRPCPort string `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	Host     string `mapstructure:"host" json:"host"`
}

type ControllerConfig struct {
	Address     string `mapstructure:"address" validate:"required" json:"address"`
	Environment string `mapstructure:"environment" validate:"required" json:"environment"`
}

type JWTConfig struct {
	JWKSURL string `mapstructure:"jwks_url" validate:"required" json:"jwks_url"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Set default values
	viper.SetDefault("server.http_port", "8080")
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("controller.address", "gateway-controller:19090")
	viper.SetDefault("controller.environment", "dev")
	viper.SetDefault("jwt.jwks_url", "http://api-server:8080/.well-known/jwks.json")

	// Support environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("AUTH_SIDECAR_")

	// If a specific config file is passed
	if len(configFile) > 0 && configFile[0] != "" {
		slog.Info(fmt.Sprintf("Loading config from %s", configFile[0]))
		viper.SetConfigFile(configFile[0])
	} else {
		// Default settings for finding the file
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("./configs/api-server")
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
		slog.Info(fmt.Sprintf("UUUUUUsing config file %s", viper.ConfigFileUsed()))
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
