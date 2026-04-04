package config

import (
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
	// Isolated instance: global viper can be mutated by other imports; Unmarshal must see the same tree as Set.
	v := viper.New()
	v.SetDefault("server.http_port", "8080")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("etcd.dial_timeout", "5s")
	v.SetDefault("jwt.keys_dir", "./secrets/keys/jwt")
	v.SetDefault("jwt.issuer", "api-gateway-api-server")
	v.SetDefault("leader_election.enabled", true)
	v.SetDefault("leader_election.key_prefix", "/api-gateway/api-server/election/leader")
	v.SetDefault("leader_election.session_ttl_seconds", 5)

	v.AutomaticEnv()
	v.SetEnvPrefix("API_SERVER_")

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
