package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	TokenResultCreated                = "created"
	TokenResultValidationBind         = "validation_bind_error"
	TokenResultValidationAppID        = "validation_missing_app_id"
	TokenResultValidationEnvironments = "validation_missing_environments"
	TokenResultValidationEmptyEnv     = "validation_empty_environment"
	TokenResultValidationExpiresAt    = "validation_expires_at"
	TokenResultForbidden              = "forbidden"
	TokenResultInternalError          = "internal_error"
)

var tokenGenerateTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "api_server_token_generate_total",
		Help: "JWT token issuance outcomes for token minting routes (e.g. POST /api/v1/tokens/edge).",
	},
	[]string{"result"},
)

// RecordTokenGenerate increments token generate counter when enabled.
func RecordTokenGenerate(enabled bool, result string) {
	if !enabled {
		return
	}
	tokenGenerateTotal.WithLabelValues(result).Inc()
}
