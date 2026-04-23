package config

import (
	"log/slog"

	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/metricshttp"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server" validate:"required" json:"server"`
	Controller ControllerConfig `mapstructure:"controller" validate:"required" json:"controller"`
	JWT        JWTConfig        `mapstructure:"jwt" validate:"required" json:"jwt"`
	// GRPCExtAuthz: TLS and observability for the ext_authz gRPC server.
	GRPCExtAuthz GRPCExtAuthzSection `mapstructure:"grpc_ext_authz" json:"grpc_ext_authz"`
	// GRPCControllerClient: TLS when dialing Controller.
	GRPCControllerClient grpcobs.ClientTLSConfig `mapstructure:"grpc_controller_client" json:"grpc_controller_client"`
	MetricsHTTP          metricshttp.Config      `mapstructure:"metrics_http" json:"metrics_http"`
	// Telemetry: OpenTelemetry trace export (optional). Merged with env; see FileBlock in the telemetry package.
	Telemetry telemetry.FileBlock `mapstructure:"telemetry" json:"telemetry"`
}

type ServerConfig struct {
	GRPCPort string `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	Host     string `mapstructure:"host" json:"host"`
}

// GRPCExtAuthzSection groups server TLS and observability for ext_authz gRPC.
type GRPCExtAuthzSection struct {
	TLS           grpcobs.ServerTLSConfig     `mapstructure:"tls" json:"tls"`
	Observability grpcobs.ObservabilityConfig `mapstructure:"observability" json:"observability"`
}

type ControllerConfig struct {
	Address     string `mapstructure:"address" validate:"required" json:"address"`
	Environment string `mapstructure:"environment" validate:"required" json:"environment"`
}

type JWTConfig struct {
	JWKSURL string `mapstructure:"jwks_url" validate:"required" json:"jwks_url"`
	// ExpectedIssuer / ExpectedAudience must match API Server Edge JWT claims (roadmap ш. 16).
	ExpectedIssuer   string `mapstructure:"expected_issuer" json:"expected_issuer"`
	ExpectedAudience string `mapstructure:"expected_audience" json:"expected_audience"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Isolated instance: global viper can be mutated by other imports; Unmarshal must see the same tree as Set.
	v := viper.New()
	v.SetDefault("server.http_port", "8080")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("jwt.jwks_url", "http://api-server:8080/.well-known/jwks-edge.json")
	v.SetDefault("jwt.expected_issuer", "api-gateway-edge")
	v.SetDefault("jwt.expected_audience", "api-gateway-edge-http")
	v.SetDefault("grpc_ext_authz.observability.reflection_enabled", true)
	v.SetDefault("grpc_ext_authz.observability.log_requests", false)
	v.SetDefault("metrics_http.enabled", false)
	v.SetDefault("metrics_http.host", "0.0.0.0")
	v.SetDefault("metrics_http.port", "9090")
	v.SetDefault("metrics_http.path", "/metrics")

	v.AutomaticEnv()
	v.SetEnvPrefix("SIDECAR_")

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
