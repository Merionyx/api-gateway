package metrics

import "testing"

func TestSetLeader(t *testing.T) {
	t.Parallel()
	SetLeader(false, true)
	SetLeader(true, true)
	SetLeader(true, false)
}
