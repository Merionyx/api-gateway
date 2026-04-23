package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Fixed reason labels for sidecar_authorization_decisions_total (low cardinality).
const (
	ReasonContractNotFound      = "contract_not_found"
	ReasonInsecureAllow         = "insecure_allow"
	ReasonMissingToken          = "missing_token"
	ReasonInvalidJWT            = "invalid_jwt"
	ReasonMissingAppID          = "missing_app_id"
	ReasonMissingEnvironments   = "missing_environments"
	ReasonEnvironmentsWrongType = "environments_wrong_type"
	ReasonEnvironmentNotString  = "environment_not_string"
	ReasonEnvNotAllowed         = "env_not_allowed"
	ReasonAccessDenied          = "access_denied"
	ReasonAllowOK               = "allow_ok"
)

const (
	ResultAllow = "allow"
	ResultDeny  = "deny"
)

var (
	authorizationDecisions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sidecar_authorization_decisions_total",
			Help: "Authorization check outcomes from ext_authz.",
		},
		[]string{"result", "reason"},
	)
	checkDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sidecar_authorization_check_duration_seconds",
			Help:    "Duration of ext_authz Check handler.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result"},
	)
)

// RecordAuthorization records decision and optional duration when metrics are enabled.
func RecordAuthorization(enabled bool, result, reason string, elapsed time.Duration) {
	if !enabled {
		return
	}
	authorizationDecisions.WithLabelValues(result, reason).Inc()
	checkDuration.WithLabelValues(result).Observe(elapsed.Seconds())
}
