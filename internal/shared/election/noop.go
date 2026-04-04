package election

// NoopGate always reports leader (single-instance or election disabled).
type NoopGate struct{}

func (NoopGate) IsLeader() bool { return true }

// LeaderChanged returns nil: leadership never flips; callers reconcile once from IsLeader().
func (NoopGate) LeaderChanged() <-chan struct{} { return nil }
