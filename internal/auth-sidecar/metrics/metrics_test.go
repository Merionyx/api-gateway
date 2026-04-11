package metrics

import (
	"testing"
	"time"
)

func TestRecordAuthorization(t *testing.T) {
	t.Parallel()
	RecordAuthorization(false, ResultAllow, ReasonAllowOK, time.Millisecond)
	RecordAuthorization(true, ResultDeny, ReasonMissingToken, time.Millisecond)
}
