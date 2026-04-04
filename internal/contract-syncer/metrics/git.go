package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	GitResultOK    = "ok"
	GitResultError = "error"
)

var (
	gitSyncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "contract_syncer_git_sync_duration_seconds",
			Help:    "Duration of gitManager.GetRepositorySnapshots per Sync call.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result"},
	)
	snapshotsProduced = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "contract_syncer_snapshots_produced",
			Help:    "Number of contract snapshots returned on successful git sync.",
			Buckets: []float64{0, 1, 2, 5, 10, 20, 50},
		},
	)
)

// RecordGitSyncDuration observes git pull phase duration.
func RecordGitSyncDuration(enabled bool, result string, d time.Duration) {
	if !enabled {
		return
	}
	gitSyncDuration.WithLabelValues(result).Observe(d.Seconds())
}

// RecordSnapshotsProduced observes snapshot count on success.
func RecordSnapshotsProduced(enabled bool, n int) {
	if !enabled {
		return
	}
	snapshotsProduced.Observe(float64(n))
}
