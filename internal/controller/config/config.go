package config

import (
	"fmt"
	"log/slog"
	"strings"

	"merionyx/api-gateway/internal/shared/etcd"

	"github.com/spf13/viper"
)

type Config struct {
	Server         ServerConfig         `mapstructure:"server" validate:"required" json:"server"`
	Etcd           etcd.EtcdConfig      `mapstructure:"etcd"`
	Environments   []EnvironmentConfig  `mapstructure:"environments" validate:"required" json:"environments"`
	Services       ServicesConfig       `mapstructure:"services" validate:"required" json:"services"`
	APIServer      APIServerConfig      `mapstructure:"api_server" json:"api_server"`
	Tenant         string               `mapstructure:"tenant" json:"tenant"`
	HA             HAConfig             `mapstructure:"ha" json:"ha"`
	LeaderElection LeaderElectionConfig `mapstructure:"leader_election" json:"leader_election"`
}

// HAConfig groups settings shared by all replicas of one logical Gateway Controller pool.
type HAConfig struct {
	// ControllerID is the id registered with API Server. Set the same value on every replica (default: hostname).
	ControllerID string `mapstructure:"controller_id" json:"controller_id"`
}

// LeaderElectionConfig elects a single replica to stream from API Server and write contract snapshots to etcd.
type LeaderElectionConfig struct {
	Enabled           bool   `mapstructure:"enabled" json:"enabled"`
	KeyPrefix         string `mapstructure:"key_prefix" json:"key_prefix"`
	Identity          string `mapstructure:"identity" json:"identity"`
	SessionTTLSeconds int    `mapstructure:"session_ttl_seconds" json:"session_ttl_seconds"`
}

type APIServerConfig struct {
	Address string `mapstructure:"address" json:"address"`
}

type ServerConfig struct {
	HTTP1Port string `mapstructure:"http1_port" validate:"required" json:"http1_port"`
	HTTP2Port string `mapstructure:"http2_port" validate:"required" json:"http2_port"`
	GRPCPort  string `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	XDSPort   string `mapstructure:"xds_port" validate:"required" json:"xds_port"`
	Host      string `mapstructure:"host" json:"host"`
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
	viper.SetDefault("server.http1_port", "8080")
	viper.SetDefault("server.http2_port", "8443")
	viper.SetDefault("server.grpc_port", "19090")
	viper.SetDefault("server.xds_port", "19091")
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("logging.enabled", false)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "[${time}] ${status} - ${method} ${path}")
	viper.SetDefault("leader_election.enabled", true)
	viper.SetDefault("leader_election.key_prefix", "/api-gateway/controller/election/leader")
	viper.SetDefault("leader_election.session_ttl_seconds", 5)

	viper.AutomaticEnv()
	viper.SetEnvPrefix("CP_")

	if len(configFile) > 0 && configFile[0] != "" {
		slog.Info(fmt.Sprintf("Loading config from %s", configFile[0]))
		viper.SetConfigFile(configFile[0])
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("./configs/control-plane")
	}

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

	if strings.TrimSpace(config.APIServer.Address) == "" {
		return nil, fmt.Errorf("api_server.address is required (gRPC address of API Server)")
	}

	return &config, nil
}
