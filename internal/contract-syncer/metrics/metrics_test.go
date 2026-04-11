package metrics

import (
	"testing"
	"time"
)

func TestRecordSync(t *testing.T) {
	t.Parallel()
	RecordSync(false, OutcomeOK, time.Second)
	RecordSync(true, OutcomeResponseError, time.Millisecond)
}

func TestRecordGitAndSnapshots(t *testing.T) {
	t.Parallel()
	RecordGitSyncDuration(false, GitResultOK, time.Second)
	RecordGitSyncDuration(true, GitResultError, time.Millisecond)
	RecordSnapshotsProduced(false, 3)
	RecordSnapshotsProduced(true, 2)
}
