package serviceapp

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestRunHTTPServerUntil_ListenError(t *testing.T) {
	err := RunHTTPServerUntil(context.Background(), &http.Server{}, "127.0.0.1:100000")
	if err == nil {
		t.Fatal("expected listen error")
	}
}

func TestRunHTTPServerUntil_CancelGracefulShutdown(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunHTTPServerUntil(ctx, srv, addr) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestRunGRPCServeUntil_Cancel(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunGRPCServeUntil(ctx, srv, lis) }()

	cancel()

	select {
	case err := <-done:
		_ = err // grpc may return ErrServerStopped or nil depending on version
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

func TestRunGRPCServeUntil_ServeReturnsBeforeCancel(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = lis.Close()
	srv := grpc.NewServer()
	ctx := context.Background()
	err = RunGRPCServeUntil(ctx, srv, lis)
	if err == nil {
		t.Fatal("expected error from Serve on closed listener")
	}
}

func TestSetDefaultLogger(t *testing.T) {
	l := NewJSONLogger()
	SetDefaultLogger(l)
	if slog.Default() != l {
		t.Fatal("default logger not set")
	}
}
