package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/grpcutil"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// spanUsecaseAPIPkg is the import path of this file for [telemetry.SpanName].
const spanUsecaseAPIPkg = "github.com/merionyx/api-gateway/internal/controller/usecase"

// leaderAPIServerStream runs the long-lived gRPC session to the API Server (this replica when
// it is the cluster leader): register, heartbeat, snapshot stream, save to schema etcd, xDS nudge.
// See [etcdFollowerWatch] for the every-replica path that watches controller etcd.
type leaderAPIServerStream struct {
	config       *config.Config
	apiAddress   string
	controllerID string
	reg          *registryEnvironmentsBuilder
	schema       interfaces.SchemaRepository
	reconciler   interfaces.EffectiveReconciler
}

func newLeaderAPIServerStream(
	cfg *config.Config,
	apiAddress, controllerID string,
	reg *registryEnvironmentsBuilder,
	schema interfaces.SchemaRepository,
	recon interfaces.EffectiveReconciler,
) *leaderAPIServerStream {
	return &leaderAPIServerStream{
		config:       cfg,
		apiAddress:   apiAddress,
		controllerID: controllerID,
		reg:          reg,
		schema:       schema,
		reconciler:   recon,
	}
}

// registerAndStream — outer loop (backoff → connect → runAPIServerSession).
// Iteration outcomes: afterRegisterAndStreamSession.
func (l *leaderAPIServerStream) registerAndStream(ctx context.Context) error {
	const (
		initialBackoff = time.Second
		maxBackoff     = 60 * time.Second
	)
	backoff := time.Duration(0)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if backoff > 0 {
			slog.Warn("Reconnecting to API Server after backoff", "address", l.apiAddress, "backoff", backoff)
			if err := grpcutil.SleepOrDone(ctx, backoff); err != nil {
				return err
			}
		}

		slog.Info("Connecting to API Server", "address", l.apiAddress)
		sessErr := l.runAPIServerSession(ctx)
		step := afterRegisterAndStreamSession(ctx, sessErr)
		if step.sessionEnd != "" {
			metrics.RecordAPIServerSessionEnd(l.config.MetricsHTTP.Enabled, step.sessionEnd)
		}
		if !step.endLoop {
			slog.Warn("API Server sync session ended", "error", sessErr)
			backoff = grpcutil.NextReconnectBackoff(backoff, initialBackoff, maxBackoff)
			continue
		}
		return step.returnErr
	}
}

func (l *leaderAPIServerStream) grpcDialOptions() ([]grpc.DialOption, error) {
	tlsOpts, err := grpcobs.DialOptions(l.config.GRPCAPIServerClient)
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
func (l *leaderAPIServerStream) runAPIServerSession(parentCtx context.Context) error {
	dialOpts, err := l.grpcDialOptions()
	if err != nil {
		return fmt.Errorf("API Server dial options: %w", err)
	}
	conn, err := grpc.NewClient(l.apiAddress, dialOpts...)
	if err != nil {
		return fmt.Errorf("dial API Server: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			slog.Debug("API Server conn close", "error", cerr)
		}
	}()

	client := pb.NewControllerRegistryServiceClient(conn)
	if err := l.registerController(parentCtx, client); err != nil {
		return fmt.Errorf("register controller: %w", err)
	}

	sessionCtx, cancelSession := context.WithCancel(parentCtx)
	defer cancelSession()

	go l.startHeartbeat(sessionCtx, client)

	streamErr := l.streamSnapshots(sessionCtx, client)
	cancelSession()
	if err := parentCtx.Err(); err != nil {
		return err
	}
	return streamErr
}

func (l *leaderAPIServerStream) registerController(ctx context.Context, client pb.ControllerRegistryServiceClient) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAPIPkg, "registerController"))
	defer span.End()
	rpcCtx := telemetry.OutgoingContextWithTrace(ctx)

	environments, report, err := l.reg.buildEnvironmentsForAPIServer(ctx)
	observeRegistryEnvironmentsBuildDegradation(ctx, l.config, registryOpRegister, &report)
	if err != nil {
		telemetry.MarkError(span, err)
		return err
	}

	_, err = client.RegisterController(rpcCtx, &pb.RegisterControllerRequest{
		ControllerId:           l.controllerID,
		Tenant:                 l.config.Tenant,
		Environments:           environments,
		RegistryPayloadVersion: RegistryPayloadVersionV1,
	})
	if err != nil {
		telemetry.MarkError(span, err)
		return err
	}

	slog.Info("registered with API server", "controller_id", l.controllerID, "environments_count", len(environments))
	return nil
}

func (l *leaderAPIServerStream) startHeartbeat(ctx context.Context, client pb.ControllerRegistryServiceClient) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hctx, hspan := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAPIPkg, "sendHeartbeat"))
			environments, report, err := l.reg.buildEnvironmentsForAPIServer(hctx)
			observeRegistryEnvironmentsBuildDegradation(ctx, l.config, registryOpHeartbeat, &report)
			if err != nil {
				telemetry.MarkError(hspan, err)
				hspan.End()
				slog.Error("build environments for heartbeat", "error", err)
				continue
			}

			rpcCtx := telemetry.OutgoingContextWithTrace(hctx)
			_, err = client.Heartbeat(rpcCtx, &pb.HeartbeatRequest{
				ControllerId:           l.controllerID,
				Environments:           environments,
				RegistryPayloadVersion: RegistryPayloadVersionV1,
			})
			telemetry.MarkError(hspan, err)
			hspan.End()
			if err != nil {
				slog.Error("send heartbeat", "error", err)
			}
		}
	}
}

func (l *leaderAPIServerStream) streamSnapshots(ctx context.Context, client pb.ControllerRegistryServiceClient) (err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAPIPkg, "streamSnapshots"))
	defer func() {
		if err != nil {
			telemetry.MarkError(span, err)
		}
		span.End()
	}()
	rpcCtx := telemetry.OutgoingContextWithTrace(ctx)
	stream, err := client.StreamSnapshots(rpcCtx, &pb.StreamSnapshotsRequest{
		ControllerId: l.controllerID,
	})
	if err != nil {
		return fmt.Errorf("open StreamSnapshots: %w", err)
	}

	slog.Info("Started receiving snapshot stream")

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("snapshot stream closed by server: %w", err)
			}
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.Canceled, codes.DeadlineExceeded:
					if ctx.Err() != nil {
						return ctx.Err()
					}
				}
			}
			return fmt.Errorf("snapshot stream recv: %w", err)
		}

		slog.Info("Received snapshots", "environment", resp.Environment, "bundle_key", resp.BundleKey, "count", len(resp.Snapshots))

		if len(resp.Snapshots) == 0 {
			slog.Debug("Skipping empty snapshot batch to avoid clearing xDS", "environment", resp.Environment, "bundle_key", resp.BundleKey)
			continue
		}

		var snapshots []sharedgit.ContractSnapshot
		for _, pbSnapshot := range resp.Snapshots {
			var apps []sharedgit.App
			if acc := pbSnapshot.GetAccess(); acc != nil {
				for _, pbApp := range acc.GetApps() {
					apps = append(apps, sharedgit.App{
						AppID:        pbApp.GetAppId(),
						Environments: pbApp.GetEnvironments(),
					})
				}
			}
			upstreamName := ""
			if u := pbSnapshot.GetUpstream(); u != nil {
				upstreamName = u.GetName()
			}
			secure := false
			if acc := pbSnapshot.GetAccess(); acc != nil {
				secure = acc.GetSecure()
			}
			snapshots = append(snapshots, sharedgit.ContractSnapshot{
				Name:                  pbSnapshot.GetName(),
				Prefix:                pbSnapshot.GetPrefix(),
				Upstream:              sharedgit.ContractUpstream{Name: upstreamName},
				AllowUndefinedMethods: pbSnapshot.GetAllowUndefinedMethods(),
				Access: sharedgit.Access{
					Secure: secure,
					Apps:   apps,
				},
			})
		}

		if err := l.saveSnapshotsToEtcd(ctx, resp.BundleKey, snapshots); err != nil {
			slog.Error("Failed to save snapshots to etcd", "error", err)
			continue
		}

		if err := l.updateXDSSnapshot(ctx, resp.Environment); err != nil {
			slog.Error("Failed to update xDS snapshot", "error", err)
		}
	}
}

func (l *leaderAPIServerStream) saveSnapshotsToEtcd(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	repository, ref, bundlePath, err := bundlekey.Parse(bundleKey)
	if err != nil {
		return err
	}

	for _, s := range snapshots {
		cs := sharedToControllerSnapshot(s)
		slog.Info("Saving snapshot to etcd", "repository", repository, "ref", ref, "path", bundlePath, "contract", s.Name)
		if err := l.schema.SaveContractSnapshot(ctx, repository, ref, bundlePath, s.Name, cs); err != nil {
			return fmt.Errorf("save snapshot %s: %w", s.Name, err)
		}
	}
	return nil
}

func (l *leaderAPIServerStream) updateXDSSnapshot(ctx context.Context, environment string) error {
	slog.Info("Updating xDS snapshot", "environment", environment)
	if l.reconciler == nil {
		return nil
	}
	// No materialized writes on follower / hot path (leader CRUD and memory rebuild use writeMaterialized).
	return l.reconciler.ReconcileOne(ctx, environment, false)
}
