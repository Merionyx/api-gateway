package config

import (
	"log/slog"
	"strings"
	"time"

	sharedetcd "github.com/merionyx/api-gateway/internal/shared/etcd"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/metricshttp"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	"github.com/spf13/viper"
)

type Config struct {
	Server         ServerConfig          `mapstructure:"server" validate:"required" json:"server"`
	Etcd           sharedetcd.EtcdConfig `mapstructure:"etcd" validate:"required" json:"etcd"`
	JWT            JWTConfig             `mapstructure:"jwt" validate:"required" json:"jwt"`
	ContractSyncer ContractSyncerConfig  `mapstructure:"contract_syncer" validate:"required" json:"contract_syncer"`
	// Readiness configures GET /ready (etcd required; Contract Syncer optional unless RequireContractSyncer).
	Readiness      ReadinessConfig      `mapstructure:"readiness" json:"readiness"`
	LeaderElection LeaderElectionConfig `mapstructure:"leader_election" json:"leader_election"`
	// GRPCRegistry: TLS and observability for the API Server gRPC registry.
	GRPCRegistry GRPCRegistrySection `mapstructure:"grpc_registry" json:"grpc_registry"`
	// GRPCContractSyncerClient: TLS when dialing Contract Syncer.
	GRPCContractSyncerClient grpcobs.ClientTLSConfig `mapstructure:"grpc_contract_syncer_client" json:"grpc_contract_syncer_client"`
	MetricsHTTP              metricshttp.Config      `mapstructure:"metrics_http" json:"metrics_http"`
	// Idempotency configures POST /v1/bundles/sync replay when Idempotency-Key is sent.
	Idempotency IdempotencyConfig `mapstructure:"idempotency" json:"idempotency"`
	// Telemetry: OpenTelemetry trace export (optional). Merged with env; see FileBlock in the telemetry package.
	Telemetry telemetry.FileBlock `mapstructure:"telemetry" json:"telemetry"`
	// Auth: etcd key layout for API keys (and future session material); insecure bootstrap is dev-only.
	Auth AuthConfig `mapstructure:"auth" json:"auth"`
}

// IdempotencyConfig controls POST /bundles/sync idempotency (memory or etcd).
type IdempotencyConfig struct {
	// Backend is "memory" (default) or "etcd" (shared across API Server replicas).
	Backend string `mapstructure:"backend" json:"backend"`
	// BundleSyncTTL is how long completed sync outcomes are retained (memory) or etcd lease TTL.
	BundleSyncTTL time.Duration `mapstructure:"bundle_sync_ttl" json:"bundle_sync_ttl"`
	// EtcdKeyPrefix is the etcd path prefix for idempotency keys (ignored when backend=memory).
	EtcdKeyPrefix string `mapstructure:"etcd_key_prefix" json:"etcd_key_prefix"`
	// Cluster optional segment to isolate keys when several logical envs share one etcd.
	Cluster string `mapstructure:"cluster" json:"cluster"`
}

// GRPCRegistrySection groups server TLS and observability for the gRPC registry listener.
type GRPCRegistrySection struct {
	TLS           grpcobs.ServerTLSConfig     `mapstructure:"tls" json:"tls"`
	Observability grpcobs.ObservabilityConfig `mapstructure:"observability" json:"observability"`
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

// ReadinessConfig controls GET /ready checks for orchestrators.
type ReadinessConfig struct {
	// RequireContractSyncer when true includes Contract Syncer ping in readiness (503 if down).
	RequireContractSyncer bool `mapstructure:"require_contract_syncer" json:"require_contract_syncer"`
}

type ServerConfig struct {
	HTTPPort string     `mapstructure:"http_port" validate:"required" json:"http_port"`
	GRPCPort string     `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	Host     string     `mapstructure:"host" json:"host"`
	CORS     CORSConfig `mapstructure:"cors" json:"cors"`
}

type JWTConfig struct {
	KeysDir string `mapstructure:"keys_dir" validate:"required" json:"keys_dir"`
	Issuer  string `mapstructure:"issuer" validate:"required" json:"issuer"`
	// APIAudience is the JWT aud claim for interactive API access tokens (Edge≠API).
	APIAudience string `mapstructure:"api_audience" json:"api_audience"`
	// EdgeKeysDir holds Edge-profile signing keys. Empty means keys_dir/edge (see auth.NewJWTUseCase).
	EdgeKeysDir string `mapstructure:"edge_keys_dir" json:"edge_keys_dir"`
	// APISigningKid / EdgeSigningKid pin which loaded key signs new tokens when multiple *.key files exist (rotation).
	// Empty means "newest private key by file mtime" within each directory.
	APISigningKid  string `mapstructure:"api_signing_kid" json:"api_signing_kid"`
	EdgeSigningKid string `mapstructure:"edge_signing_kid" json:"edge_signing_kid"`
	// EdgeIssuer / EdgeAudience are iss/aud for POST /v1/tokens/edge (data-plane / ExtAuthz profile).
	EdgeIssuer   string `mapstructure:"edge_issuer" json:"edge_issuer"`
	EdgeAudience string `mapstructure:"edge_audience" json:"edge_audience"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Isolated instance: global viper can be mutated by other imports; Unmarshal must see the same tree as Set.
	v := viper.New()
	v.SetDefault("server.http_port", "8080")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("etcd.dial_timeout", "5s")
	v.SetDefault("jwt.keys_dir", "./secrets/keys/jwt")
	v.SetDefault("jwt.issuer", "api-gateway-api-server")
	v.SetDefault("jwt.api_audience", "api-gateway-api-http")
	v.SetDefault("jwt.edge_issuer", "api-gateway-edge")
	v.SetDefault("jwt.edge_audience", "api-gateway-edge-http")
	v.SetDefault("leader_election.enabled", true)
	v.SetDefault("leader_election.key_prefix", "/api-gateway/api-server/election/leader")
	v.SetDefault("leader_election.session_ttl_seconds", 5)
	v.SetDefault("grpc_registry.observability.reflection_enabled", true)
	v.SetDefault("grpc_registry.observability.log_requests", false)
	v.SetDefault("metrics_http.enabled", false)
	v.SetDefault("metrics_http.host", "0.0.0.0")
	v.SetDefault("metrics_http.port", "9090")
	v.SetDefault("metrics_http.path", "/metrics")
	v.SetDefault("readiness.require_contract_syncer", false)
	v.SetDefault("idempotency.backend", "memory")
	v.SetDefault("idempotency.bundle_sync_ttl", 24*time.Hour)
	v.SetDefault("idempotency.etcd_key_prefix", "/api-gateway/api-server/idempotency/v1")
	v.SetDefault("idempotency.cluster", "")
	v.SetDefault("auth.etcd_key_prefix", "/api-gateway/api-server/auth/v1")
	v.SetDefault("auth.environment", "development")
	v.SetDefault("auth.allow_insecure_bootstrap", false)
	v.SetDefault("auth.login_intent_lease_ttl", 15*time.Minute)
	v.SetDefault("auth.interactive_access_token_ttl", DefaultInteractiveAccessTokenTTL)
	v.SetDefault("auth.interactive_access_token_max_ttl", DefaultInteractiveAccessTokenMaxTTL)
	v.SetDefault("auth.interactive_refresh_token_ttl", DefaultInteractiveRefreshTokenTTL)
	v.SetDefault("auth.interactive_refresh_token_max_ttl", DefaultInteractiveRefreshTokenMaxTTL)
	v.SetDefault("auth.idp_access_cache_opaque_max_ttl", 2*time.Minute)
	// Browser CORS : explicit dev defaults — no "*" (prod Helm must list real UI origins).
	v.SetDefault("server.cors.allow_origins", DevCORSAllowOrigins())
	v.SetDefault("server.cors.insecure_allow_wildcard", false)

	v.AutomaticEnv()
	v.SetEnvPrefix("API_SERVER_")
	// Without this, nested keys map to env names containing dots (e.g. API_SERVER_AUTH.SESSION_KEK_BASE64),
	// which Kubernetes cannot set — Helm uses API_SERVER_AUTH_SESSION_KEK_BASE64 instead.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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

	ApplySessionKekFromEnv(&config)
	ApplyOIDCProviderSecretsFromEnv(&config)

	ApplyCORSDevDefaults(&config)

	if err := ValidateServerCORS(config.Server.CORS, config.Auth.Environment); err != nil {
		return nil, err
	}
	if err := ValidateInteractiveTokenTTLPolicy(
		config.Auth.InteractiveAccessTokenTTL,
		config.Auth.InteractiveAccessTokenMaxTTL,
		config.Auth.InteractiveRefreshTokenTTL,
		config.Auth.InteractiveRefreshTokenMaxTTL,
	); err != nil {
		return nil, err
	}
	if err := ValidateOIDCExternalBaseURL(config.Auth.OIDCExternalBaseURL, config.Auth.OIDCProviders); err != nil {
		return nil, err
	}
	if err := ValidateAuthorizationConfig(config.Auth.Authorization); err != nil {
		return nil, err
	}

	return &config, nil
}
