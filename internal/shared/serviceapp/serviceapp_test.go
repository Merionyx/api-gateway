package serviceapp

import (
	"context"
	"testing"
	"time"
)

func TestNewJSONLogger(t *testing.T) {
	l := NewJSONLogger()
	if l == nil {
		t.Fatal("nil logger")
	}
}

func TestWaitSignalOrContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		WaitSignalOrContext(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
