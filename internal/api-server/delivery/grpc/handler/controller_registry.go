package handler

import (
	"context"
	"log/slog"

	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/domain/models"
	pb "merionyx/api-gateway/pkg/api/controller_registry/v1"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type ControllerRegistryHandler struct {
	pb.UnimplementedControllerRegistryServiceServer
	registryUseCase interfaces.ControllerRegistryUseCase
}

func NewControllerRegistryHandler(registryUseCase interfaces.ControllerRegistryUseCase) *ControllerRegistryHandler {
	return &ControllerRegistryHandler{
		registryUseCase: registryUseCase,
	}
}

func (h *ControllerRegistryHandler) RegisterController(ctx context.Context, req *pb.RegisterControllerRequest) (*pb.RegisterControllerResponse, error) {
	slog.Info("Received register controller request", "controller_id", req.ControllerId, "tenant", req.Tenant)

	var environments []models.EnvironmentInfo
	for _, pbEnv := range req.Environments {
		var bundles []models.BundleInfo
		for _, pbBundle := range pbEnv.Bundles {
			bundles = append(bundles, models.BundleInfo{
				Name:       pbBundle.Name,
				Repository: pbBundle.Repository,
				Ref:        pbBundle.Ref,
				Path:       pbBundle.Path,
			})
		}
		environments = append(environments, models.EnvironmentInfo{
			Name:    pbEnv.Name,
			Bundles: bundles,
		})
	}

	info := models.ControllerInfo{
		ControllerID: req.ControllerId,
		Tenant:       req.Tenant,
		Environments: environments,
	}

	if err := h.registryUseCase.RegisterController(ctx, info); err != nil {
		slog.Error("Failed to register controller", "error", err)
		return &pb.RegisterControllerResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.RegisterControllerResponse{
		Success: true,
	}, nil
}

type snapshotStreamWrapper struct {
	stream pb.ControllerRegistryService_StreamSnapshotsServer
}

func (w *snapshotStreamWrapper) Send(environment, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	var pbSnapshots []*pb.ContractSnapshot
	for _, snapshot := range snapshots {
		var pbApps []*pb.App
		for _, app := range snapshot.Access.Apps {
			pbApps = append(pbApps, &pb.App{
				AppId:        app.AppID,
				Environments: app.Environments,
			})
		}

		pbSnapshots = append(pbSnapshots, &pb.ContractSnapshot{
			Name:   snapshot.Name,
			Prefix: snapshot.Prefix,
			Upstream: &pb.ContractUpstream{
				Name: snapshot.Upstream.Name,
			},
			AllowUndefinedMethods: snapshot.AllowUndefinedMethods,
			Access: &pb.Access{
				Secure: snapshot.Access.Secure,
				Apps:   pbApps,
			},
		})
	}

	return w.stream.Send(&pb.StreamSnapshotsResponse{
		Environment: environment,
		BundleKey:   bundleKey,
		Snapshots:   pbSnapshots,
	})
}

func (h *ControllerRegistryHandler) StreamSnapshots(req *pb.StreamSnapshotsRequest, stream pb.ControllerRegistryService_StreamSnapshotsServer) error {
	slog.Info("Starting snapshot stream", "controller_id", req.ControllerId)

	wrapper := &snapshotStreamWrapper{stream: stream}
	
	if err := h.registryUseCase.StreamSnapshots(stream.Context(), req.ControllerId, wrapper); err != nil {
		slog.Error("Stream error", "error", err)
		return err
	}

	return nil
}

func (h *ControllerRegistryHandler) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	slog.Debug("Received heartbeat", "controller_id", req.ControllerId)

	var environments []models.EnvironmentInfo
	for _, pbEnv := range req.Environments {
		var bundles []models.BundleInfo
		for _, pbBundle := range pbEnv.Bundles {
			bundles = append(bundles, models.BundleInfo{
				Name:       pbBundle.Name,
				Repository: pbBundle.Repository,
				Ref:        pbBundle.Ref,
				Path:       pbBundle.Path,
			})
		}
		environments = append(environments, models.EnvironmentInfo{
			Name:    pbEnv.Name,
			Bundles: bundles,
		})
	}

	if err := h.registryUseCase.Heartbeat(ctx, req.ControllerId, environments); err != nil {
		slog.Error("Failed to process heartbeat", "error", err)
		return &pb.HeartbeatResponse{
			Success: false,
		}, nil
	}

	return &pb.HeartbeatResponse{
		Success: true,
	}, nil
}
