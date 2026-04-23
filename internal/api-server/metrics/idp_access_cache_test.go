package metrics

import "testing"

func TestRecordIdpAccessCacheEvent_noopWhenDisabled(t *testing.T) {
	t.Parallel()
	RecordIdpAccessCacheEvent(false, IdpAccessCacheHit)
}

func TestRecordIdpAccessCacheEvent_enabled(t *testing.T) {
	t.Parallel()
	RecordIdpAccessCacheEvent(true, IdpAccessCacheHit)
	RecordIdpAccessCacheEvent(true, IdpAccessCacheMiss)
	RecordIdpAccessCacheEvent(true, IdpAccessCachePut)
	RecordIdpAccessCacheEvent(true, IdpAccessCacheInvalidate)
}
