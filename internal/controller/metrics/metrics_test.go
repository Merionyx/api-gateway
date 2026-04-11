package metrics

import (
	"testing"
	"time"
)

func TestRecordXDSAndSession(t *testing.T) {
	t.Parallel()
	RecordXDSnapshotUpdate(false, XDSResultOK)
	RecordXDSnapshotUpdate(true, XDSResultError)
	RecordAPIServerSessionEnd(false, SessionReasonCanceled)
	RecordAPIServerSessionEnd(true, SessionReasonError)
}

func TestEtcdAndRebuildMetrics(t *testing.T) {
	t.Parallel()
	AddEtcdWatchEvents(false, 2)
	AddEtcdWatchEvents(true, 2)
	AddEtcdWatchEvents(true, 0)
	RecordEtcdWatchError(false)
	RecordEtcdWatchError(true)
	RecordXDSRebuildFlush(true, RebuildPhaseInitial, XDSResultOK)
	ObserveXDSRebuildDuration(true, RebuildPhaseDebounced, time.Millisecond)
}

func TestAuthSchemaXDSStream(t *testing.T) {
	t.Parallel()
	ObserveAuthBuildAccessConfig(false, time.Second)
	ObserveAuthBuildAccessConfig(true, time.Millisecond)
	RecordBundleEnvIndexRebuild(false)
	RecordBundleEnvIndexRebuild(true)
	RecordSchemaListCacheHit(true, true)
	RecordSchemaListCacheHit(true, false)
	XDSStreamOpened(true)
	XDSStreamClosed(true)
	RecordXDSStreamRequest(true, "type.googleapis.com/envoy.config.listener.v3.Listener")
}

func TestNormalizeXDSResourceType(t *testing.T) {
	t.Parallel()
	if got := NormalizeXDSResourceType(""); got != "unknown" {
		t.Fatalf("%q", got)
	}
	if got := NormalizeXDSResourceType("x/y/z/foo.Bar"); got != "foo.Bar" {
		t.Fatalf("%q", got)
	}
}
