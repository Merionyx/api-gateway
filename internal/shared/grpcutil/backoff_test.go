package grpcutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNextReconnectBackoff(t *testing.T) {
	initial := 400 * time.Millisecond
	maximum := 10 * time.Second

	tests := []struct {
		current time.Duration
		want    time.Duration
	}{
		{0, initial},
		{-1 * time.Second, initial},
		{initial, 800 * time.Millisecond},
		{5 * time.Second, 10 * time.Second},
		{maximum, maximum},
		{8 * time.Second, maximum},
	}
	for _, tt := range tests {
		got := NextReconnectBackoff(tt.current, initial, maximum)
		if got != tt.want {
			t.Errorf("NextReconnectBackoff(%v, %v, %v) = %v, want %v", tt.current, initial, maximum, got, tt.want)
		}
	}
}

func TestSleepOrDoneZeroOrNegative(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := SleepOrDone(ctx, 0); err != nil {
		t.Errorf("SleepOrDone(ctx, 0) = %v, want nil", err)
	}
	if err := SleepOrDone(ctx, -time.Second); err != nil {
		t.Errorf("SleepOrDone(ctx, -1s) = %v, want nil", err)
	}
}

func TestSleepOrDoneCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SleepOrDone(ctx, time.Minute)
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

func TestSleepOrDoneDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	err := SleepOrDone(ctx, time.Minute)
	if err == nil {
		t.Fatal("expected deadline exceeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want context.DeadlineExceeded", err)
	}
}

func TestSleepOrDoneCompletes(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	if err := SleepOrDone(ctx, 15*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 10*time.Millisecond {
		t.Error("expected to wait roughly 15ms")
	}
}
