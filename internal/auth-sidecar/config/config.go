package config

import (
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
	// Isolated instance: global viper can be mutated by other imports; Unmarshal must see the same tree as Set.
	v := viper.New()
	v.SetDefault("server.http_port", "8080")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("jwt.jwks_url", "http://api-server:8080/.well-known/jwks.json")

	v.AutomaticEnv()
	v.SetEnvPrefix("AUTH_SIDECAR_")

	if len(configFile) > 0 && configFile[0] != "" {
		slog.Info("Loading config from explicit path", "path", configFile[0])
		v.SetConfigFile(configFile[0])
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("./configs/api-server")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Info("Config file not found, using defaults and environment variables")
		} else {
			slog.Error("Error reading config file", "error", err)
			return nil, err
		}
	} else {
		slog.Info("Using config file", "path", v.ConfigFileUsed())
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
