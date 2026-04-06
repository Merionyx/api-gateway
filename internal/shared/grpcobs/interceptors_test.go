package grpcobs

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024

type panicUnaryHealth struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (panicUnaryHealth) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	panic("test panic")
}

type panicStreamHealth struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (panicStreamHealth) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	panic("test panic")
}

func testGRPCDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
}

func runBufServer(t *testing.T, register func(*grpc.Server)) (*grpc.ClientConn, func()) {
	t.Helper()
	sopts, err := ServerOptions(nil, ObservabilityConfig{LogRequests: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(sopts...)
	register(srv)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(testGRPCDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
	}
}

func TestInterceptors_UnaryOK(t *testing.T) {
	conn, cleanup := runBufServer(t, func(s *grpc.Server) {
		grpc_health_v1.RegisterHealthServer(s, health.NewServer())
	})
	defer cleanup()
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := cli.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterceptors_UnaryErrorStatus(t *testing.T) {
	type errHealth struct {
		grpc_health_v1.UnimplementedHealthServer
	}
	conn, cleanup := runBufServer(t, func(s *grpc.Server) {
		grpc_health_v1.RegisterHealthServer(s, errHealth{})
	})
	defer cleanup()
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := cli.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) == codes.OK {
		t.Fatal("expected non-OK")
	}
}

func TestInterceptors_UnaryPanicRecover(t *testing.T) {
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })
	slog.SetDefault(slog.New(slog.NewTextHandler(ioDiscard{}, &slog.HandlerOptions{Level: slog.LevelError})))

	conn, cleanup := runBufServer(t, func(s *grpc.Server) {
		grpc_health_v1.RegisterHealthServer(s, panicUnaryHealth{})
	})
	defer cleanup()
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := cli.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err == nil {
		t.Fatal("expected error after panic")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestInterceptors_LogOffMetricsOff(t *testing.T) {
	sopts, err := ServerOptions(nil, ObservabilityConfig{}, false)
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(sopts...)
	grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(testGRPCDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterceptors_StreamMetricsOnLogOff(t *testing.T) {
	sopts, err := ServerOptions(nil, ObservabilityConfig{}, true)
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(sopts...)
	grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(testGRPCDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := cli.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
}

func TestInterceptors_StreamOK(t *testing.T) {
	conn, cleanup := runBufServer(t, func(s *grpc.Server) {
		grpc_health_v1.RegisterHealthServer(s, health.NewServer())
	})
	defer cleanup()
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := cli.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
}

type errWatchHealth struct {
	grpc_health_v1.UnimplementedHealthServer
}

func TestInterceptors_StreamError(t *testing.T) {
	conn, cleanup := runBufServer(t, func(s *grpc.Server) {
		grpc_health_v1.RegisterHealthServer(s, errWatchHealth{})
	})
	defer cleanup()
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := cli.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInterceptors_StreamMetricsOffLogOn(t *testing.T) {
	sopts, err := ServerOptions(nil, ObservabilityConfig{LogRequests: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(sopts...)
	grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(testGRPCDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := cli.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}

	srv2 := grpc.NewServer(sopts...)
	grpc_health_v1.RegisterHealthServer(srv2, errWatchHealth{})
	lis2 := bufconn.Listen(bufSize)
	go func() { _ = srv2.Serve(lis2) }()
	t.Cleanup(func() { srv2.Stop() })
	conn2, err := grpc.NewClient("passthrough:///bufnet2",
		grpc.WithContextDialer(testGRPCDialer(lis2)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn2.Close() })
	cli2 := grpc_health_v1.NewHealthClient(conn2)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	st2, err := cli2.Watch(ctx2, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st2.Recv(); err == nil {
		t.Fatal("expected error")
	}
}

func TestInterceptors_StreamPanicRecover(t *testing.T) {
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })
	slog.SetDefault(slog.New(slog.NewTextHandler(ioDiscard{}, &slog.HandlerOptions{Level: slog.LevelError})))

	conn, cleanup := runBufServer(t, func(s *grpc.Server) {
		grpc_health_v1.RegisterHealthServer(s, panicStreamHealth{})
	})
	defer cleanup()
	cli := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := cli.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}
