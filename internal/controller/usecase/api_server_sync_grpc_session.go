package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/shared/grpcobs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func (uc *APIServerSyncUseCase) grpcDialOptions() ([]grpc.DialOption, error) {
	tlsOpts, err := grpcobs.DialOptions(uc.config.GRPCAPIServerClient)
	if err != nil {
		return nil, err
	}
	return append(tlsOpts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	), nil
}

// runAPIServerSession uses one connection: register, heartbeat goroutine, block on snapshot stream.
func (uc *APIServerSyncUseCase) runAPIServerSession(parentCtx context.Context) error {
	dialOpts, err := uc.grpcDialOptions()
	if err != nil {
		return fmt.Errorf("API Server dial options: %w", err)
	}
	conn, err := grpc.NewClient(uc.apiServerAddress, dialOpts...)
	if err != nil {
		return fmt.Errorf("dial API Server: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			slog.Debug("API Server conn close", "error", cerr)
		}
	}()

	client := pb.NewControllerRegistryServiceClient(conn)
	if err := uc.registerController(parentCtx, client); err != nil {
		return fmt.Errorf("register controller: %w", err)
	}

	sessionCtx, cancelSession := context.WithCancel(parentCtx)
	defer cancelSession()

	go uc.startHeartbeat(sessionCtx, client)

	streamErr := uc.streamSnapshots(sessionCtx, client)
	cancelSession()
	if err := parentCtx.Err(); err != nil {
		return err
	}
	return streamErr
}

func (uc *APIServerSyncUseCase) registerController(ctx context.Context, client pb.ControllerRegistryServiceClient) error {
	environments, err := uc.buildEnvironmentsForAPIServer(ctx)
	if err != nil {
		return err
	}

	_, err = client.RegisterController(ctx, &pb.RegisterControllerRequest{
		ControllerId: uc.controllerID,
		Tenant:       uc.config.Tenant,
		Environments: environments,
	})
	if err != nil {
		return err
	}

	slog.Info("registered with API server", "controller_id", uc.controllerID, "environments_count", len(environments))
	return nil
}

func (uc *APIServerSyncUseCase) startHeartbeat(ctx context.Context, client pb.ControllerRegistryServiceClient) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			environments, err := uc.buildEnvironmentsForAPIServer(ctx)
			if err != nil {
				slog.Error("build environments for heartbeat", "error", err)
				continue
			}

			_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{
				ControllerId: uc.controllerID,
				Environments: environments,
			})
			if err != nil {
				slog.Error("send heartbeat", "error", err)
			}
		}
	}
}
