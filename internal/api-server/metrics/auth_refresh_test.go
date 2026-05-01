package metrics

import "testing"

func TestRecordAuthRefresh_noopWhenDisabled(t *testing.T) {
	t.Parallel()
	RecordAuthRefresh(false, AuthRefreshDegraded)
}

func TestRecordAuthRefresh_enabled(t *testing.T) {
	t.Parallel()
	RecordAuthRefresh(true, AuthRefreshIDPUp)
	RecordAuthRefresh(true, AuthRefreshDegraded)
}
