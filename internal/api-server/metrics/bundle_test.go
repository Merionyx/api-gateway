package metrics

import (
	"testing"
	"time"
)

func TestRecordBundleMetrics(t *testing.T) {
	t.Parallel()
	RecordBundleSyncOutcome(false, BundleOutcomeSuccess)
	RecordBundleSyncOutcome(true, BundleOutcomeSuccess)
	RecordBundleSyncAttempt(true, BundleAttemptOK)
	RecordBundleSyncDuration(false, time.Second)
	RecordBundleSyncDuration(true, time.Millisecond)
	RecordBundleEtcdWrite(true, true)
	RecordBundleEtcdWrite(true, false)
}
