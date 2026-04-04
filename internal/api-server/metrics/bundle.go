package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	BundleOutcomeSuccess     = "success"
	BundleOutcomeRejected    = "rejected"
	BundleOutcomeFailed      = "failed"
	BundleOutcomeCtxCanceled = "ctx_canceled"

	BundleAttemptOK            = "ok"
	BundleAttemptRPCError      = "rpc_error"
	BundleAttemptResponseError = "response_error"
	BundleAttemptSaveError     = "save_error"
)

var (
	bundleSyncTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_server_bundle_sync_total",
			Help: "Bundle SyncBundle outcomes (end-to-end).",
		},
		[]string{"outcome"},
	)
	bundleSyncAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_server_bundle_sync_attempts_total",
			Help: "Per-attempt results inside SyncBundle retry loop.",
		},
		[]string{"result"},
	)
	bundleSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "api_server_bundle_sync_duration_seconds",
			Help:    "Duration of successful SyncBundle (all attempts).",
			Buckets: prometheus.DefBuckets,
		},
	)
	bundleEtcdWrite = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_server_bundle_etcd_write_total",
			Help: "Etcd snapshot writes after successful contract syncer response.",
		},
		[]string{"changed"},
	)
)

// RecordBundleSyncOutcome records final SyncBundle outcome.
func RecordBundleSyncOutcome(enabled bool, outcome string) {
	if !enabled {
		return
	}
	bundleSyncTotal.WithLabelValues(outcome).Inc()
}

// RecordBundleSyncAttempt records one syncBundleOnce attempt result.
func RecordBundleSyncAttempt(enabled bool, result string) {
	if !enabled {
		return
	}
	bundleSyncAttempts.WithLabelValues(result).Inc()
}

// RecordBundleSyncDuration observes successful SyncBundle latency.
func RecordBundleSyncDuration(enabled bool, d time.Duration) {
	if !enabled {
		return
	}
	bundleSyncDuration.Observe(d.Seconds())
}

// RecordBundleEtcdWrite records whether SaveSnapshots changed keys.
func RecordBundleEtcdWrite(enabled bool, written bool) {
	if !enabled {
		return
	}
	bundleEtcdWrite.WithLabelValues(strconv.FormatBool(written)).Inc()
}
