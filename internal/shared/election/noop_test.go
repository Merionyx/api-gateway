package election

import "testing"

func TestNoopGate_IsLeader(t *testing.T) {
	var g NoopGate
	if !g.IsLeader() {
		t.Fatal("NoopGate should always be leader")
	}
	if g.LeaderChanged() != nil {
		t.Fatal("LeaderChanged should be nil")
	}
}
