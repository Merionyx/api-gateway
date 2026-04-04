package grpcobs

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type interceptorOpts struct {
	metrics bool
	log     bool
}

func chainUnary(opts interceptorOpts) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("grpc unary panic", "method", info.FullMethod, "recover", r)
				err = status.Errorf(codes.Internal, "internal error")
			}
			code := codes.OK
			if err != nil {
				code = status.Code(err)
			}
			if opts.metrics {
				serverHandled.WithLabelValues(info.FullMethod, code.String()).Inc()
				serverDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
			}
			if opts.log {
				if err != nil && code != codes.OK {
					slog.Warn("grpc unary", "method", info.FullMethod, "duration", time.Since(start), "code", code.String(), "error", err)
				} else {
					slog.Debug("grpc unary", "method", info.FullMethod, "duration", time.Since(start), "code", code.String())
				}
			}
		}()
		resp, err = handler(ctx, req)
		return resp, err
	}
}

func chainStream(opts interceptorOpts) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("grpc stream panic", "method", info.FullMethod, "recover", r)
				err = status.Errorf(codes.Internal, "internal error")
			}
			code := codes.OK
			if err != nil {
				code = status.Code(err)
			}
			if opts.metrics {
				serverHandled.WithLabelValues(info.FullMethod, code.String()).Inc()
				serverDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
			}
			if opts.log {
				if err != nil && code != codes.OK {
					slog.Warn("grpc stream", "method", info.FullMethod, "duration", time.Since(start), "code", code.String(), "error", err)
				} else {
					slog.Debug("grpc stream", "method", info.FullMethod, "duration", time.Since(start), "code", code.String())
				}
			}
		}()
		err = handler(srv, ss)
		return err
	}
}
