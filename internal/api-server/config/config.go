package config

import (
	"fmt"
	"log/slog"

	sharedetcd "merionyx/api-gateway/internal/shared/etcd"

	"github.com/spf13/viper"
)

type Config struct {
	Server         ServerConfig          `mapstructure:"server" validate:"required" json:"server"`
	Etcd           sharedetcd.EtcdConfig `mapstructure:"etcd" validate:"required" json:"etcd"`
	JWT            JWTConfig             `mapstructure:"jwt" validate:"required" json:"jwt"`
	ContractSyncer ContractSyncerConfig  `mapstructure:"contract_syncer" validate:"required" json:"contract_syncer"`
	LeaderElection LeaderElectionConfig  `mapstructure:"leader_election" json:"leader_election"`
}

// LeaderElectionConfig gates single-writer work (bundle pull from Contract Syncer) to one API Server replica.
type LeaderElectionConfig struct {
	Enabled           bool   `mapstructure:"enabled" json:"enabled"`
	KeyPrefix         string `mapstructure:"key_prefix" json:"key_prefix"`
	Identity          string `mapstructure:"identity" json:"identity"`
	SessionTTLSeconds int    `mapstructure:"session_ttl_seconds" json:"session_ttl_seconds"`
}

type ContractSyncerConfig struct {
	Address string `mapstructure:"address" validate:"required" json:"address"`
}

type ServerConfig struct {
	HTTPPort string `mapstructure:"http_port" validate:"required" json:"http_port"`
	GRPCPort string `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	Host     string `mapstructure:"host" json:"host"`
}

type JWTConfig struct {
	KeysDir string `mapstructure:"keys_dir" validate:"required" json:"keys_dir"`
	Issuer  string `mapstructure:"issuer" validate:"required" json:"issuer"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Set default values
	viper.SetDefault("server.http_port", "8080")
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("etcd.dial_timeout", "5s")
	viper.SetDefault("jwt.keys_dir", "./secrets/keys/jwt")
	viper.SetDefault("jwt.issuer", "api-gateway-api-server")
	viper.SetDefault("leader_election.enabled", true)
	viper.SetDefault("leader_election.key_prefix", "/api-gateway/api-server/election/leader")
	viper.SetDefault("leader_election.session_ttl_seconds", 5)

	// Support environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("API_SERVER_")

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
		slog.Info(fmt.Sprintf("Using config file %s", viper.ConfigFileUsed()))
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
