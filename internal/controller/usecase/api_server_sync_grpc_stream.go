package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (uc *APIServerSyncUseCase) streamSnapshots(ctx context.Context, client pb.ControllerRegistryServiceClient) error {
	stream, err := client.StreamSnapshots(ctx, &pb.StreamSnapshotsRequest{
		ControllerId: uc.controllerID,
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

		if err := uc.saveSnapshotsToEtcd(ctx, resp.BundleKey, snapshots); err != nil {
			slog.Error("Failed to save snapshots to etcd", "error", err)
			continue
		}

		if err := uc.updateXDSSnapshot(ctx, resp.Environment); err != nil {
			slog.Error("Failed to update xDS snapshot", "error", err)
		}
	}
}
