package election

// NoopGate always reports leader (single-instance or election disabled).
type NoopGate struct{}

func (NoopGate) IsLeader() bool { return true }
