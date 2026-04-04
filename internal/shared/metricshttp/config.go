package metricshttp

// Config is the same shape in every binary (mapstructure keys match across services).
type Config struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled"`
	Host    string `mapstructure:"host" json:"host"`
	Port    string `mapstructure:"port" json:"port"`
	Path    string `mapstructure:"path" json:"path"`
}
