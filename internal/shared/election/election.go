package election

// LeaderGate reports whether this process may perform leader-only work.
type LeaderGate interface {
	IsLeader() bool
	// LeaderChanged delivers a signal after each transition that may change IsLeader().
	// The receiver should call IsLeader() after reading from the channel. At most one
	// goroutine should range on this channel.
	LeaderChanged() <-chan struct{}
}
