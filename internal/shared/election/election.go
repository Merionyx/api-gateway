package election

// LeaderGate reports whether this process may perform leader-only work.
type LeaderGate interface {
	IsLeader() bool
}
