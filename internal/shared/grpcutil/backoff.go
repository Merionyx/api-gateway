package grpcutil

import (
	"context"
	"time"
)

// SleepOrDone waits for d or until ctx is cancelled.
func SleepOrDone(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// NextReconnectBackoff doubles delay up to max (starts from initial).
func NextReconnectBackoff(current, initial, max time.Duration) time.Duration {
	if current <= 0 {
		return initial
	}
	next := current * 2
	if next > max {
		return max
	}
	return next
}
