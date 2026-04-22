package election

import (
	"testing"
)

func TestNewEtcdGate_DefaultTTL(t *testing.T) {
	g := NewEtcdGate(nil, "/p", "id", 0)
	if g == nil {
		t.Fatal("nil gate")
	}
	// ttlSec <= 0 -> 5 inside constructor; field is unexported — exercise Run noop: client nil will loop warn
	_ = g.IsLeader()
}

func TestNewEtcdGate_ExplicitTTL(t *testing.T) {
	t.Parallel()
	g := NewEtcdGate(nil, "/prefix", "id-2", 42)
	if g == nil {
		t.Fatal("nil")
	}
	_ = g.LeaderChanged()
	_ = g.IsLeader()
}
