package grpcobs

// ServerTLSConfig enables TLS on a gRPC server. If ClientCAFile is set, clients must present
// a certificate signed by that CA (mTLS).
type ServerTLSConfig struct {
	Enabled      bool   `mapstructure:"enabled" json:"enabled"`
	CertFile     string `mapstructure:"cert_file" json:"cert_file"`
	KeyFile      string `mapstructure:"key_file" json:"key_file"`
	ClientCAFile string `mapstructure:"client_ca_file" json:"client_ca_file"`
}

// ClientTLSConfig enables TLS on outbound gRPC. CAFile verifies the server; CertFile/KeyFile add mTLS.
type ClientTLSConfig struct {
	Enabled    bool   `mapstructure:"enabled" json:"enabled"`
	CAFile     string `mapstructure:"ca_file" json:"ca_file"`
	CertFile   string `mapstructure:"cert_file" json:"cert_file"`
	KeyFile    string `mapstructure:"key_file" json:"key_file"`
	ServerName string `mapstructure:"server_name" json:"server_name"`
}

// ObservabilityConfig toggles gRPC reflection and request logging.
// Prometheus gRPC metrics are recorded when the binary sets metrics_http.enabled and passes recordPrometheus into ServerOptions.
type ObservabilityConfig struct {
	ReflectionEnabled bool `mapstructure:"reflection_enabled" json:"reflection_enabled"`
	LogRequests       bool `mapstructure:"log_requests" json:"log_requests"`
}
