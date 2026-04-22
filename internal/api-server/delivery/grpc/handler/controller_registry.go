package handler

import (
	"context"
	"log/slog"

	commonv1 "github.com/merionyx/api-gateway/pkg/grpc/common/v1"
	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/grpc/grpcerr"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type ControllerRegistryHandler struct {
	pb.UnimplementedControllerRegistryServiceServer
	registryUseCase interfaces.ControllerRegistryUseCase
	metricsEnabled  bool
}

func NewControllerRegistryHandler(registryUseCase interfaces.ControllerRegistryUseCase, metricsEnabled bool) *ControllerRegistryHandler {
	return &ControllerRegistryHandler{
		registryUseCase: registryUseCase,
		metricsEnabled:  metricsEnabled,
	}
}

// registryConfigSourceString maps proto enum to stable JSON in etcd (see OpenAPI / ADR 0001).
func registryConfigSourceString(s pb.ConfigSource) string {
	switch s {
	case pb.ConfigSource_CONFIG_SOURCE_FILE:
		return "file"
	case pb.ConfigSource_CONFIG_SOURCE_KUBERNETES:
		return "kubernetes"
	case pb.ConfigSource_CONFIG_SOURCE_ETCD_GRPC:
		return "etcd_grpc"
	default:
		return ""
	}
}

func (h *ControllerRegistryHandler) RegisterController(ctx context.Context, req *pb.RegisterControllerRequest) (*pb.RegisterControllerResponse, error) {
	slog.Info("Received register controller request", "controller_id", req.ControllerId, "tenant", req.Tenant)

	var environments []models.EnvironmentInfo
	for _, pbEnv := range req.Environments {
		var bundles []models.BundleInfo
		for _, pbBundle := range pbEnv.Bundles {
			bi := models.BundleInfo{
				Name:       pbBundle.Name,
				Repository: pbBundle.Repository,
				Ref:        pbBundle.Ref,
				Path:       pbBundle.Path,
			}
			if p := pbBundle.GetProvenance(); p != nil {
				bi.ConfigSource = registryConfigSourceString(p.GetSource())
			}
			bundles = append(bundles, bi)
		}
		var services []models.ServiceInfo
		for _, ps := range pbEnv.GetServices() {
			si := models.ServiceInfo{Name: ps.GetName(), Upstream: ps.GetUpstream()}
			if p := ps.GetProvenance(); p != nil {
				si.ConfigSource = registryConfigSourceString(p.GetSource())
			}
			services = append(services, si)
		}
		env := models.EnvironmentInfo{Name: pbEnv.Name, Bundles: bundles, Services: services}
		if pbEnv.GetSourcesFingerprint() != "" {
			env.SourcesFingerprint = pbEnv.GetSourcesFingerprint()
		}
		if pbEnv.EffectiveGeneration != nil {
			g := pbEnv.GetEffectiveGeneration()
			env.EffectiveGeneration = &g
		}
		if s := pbEnv.GetEnvironmentConfigSource(); s != pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED {
			env.EnvironmentConfigSource = registryConfigSourceString(s)
		}
		environments = append(environments, env)
	}

	info := models.ControllerInfo{
		ControllerID: req.ControllerId,
		Tenant:       req.Tenant,
		Environments: environments,
	}

	if err := h.registryUseCase.RegisterController(ctx, info); err != nil {
		slog.Error("Failed to register controller", "error", err)
		return nil, grpcerr.Status(h.metricsEnabled, err)
	}

	return &pb.RegisterControllerResponse{Success: true}, nil
}

type snapshotStreamWrapper struct {
	stream pb.ControllerRegistryService_StreamSnapshotsServer
}

func (w *snapshotStreamWrapper) Send(environment, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	var pbSnapshots []*commonv1.ContractSnapshot
	for _, snapshot := range snapshots {
		var pbApps []*commonv1.App
		for _, app := range snapshot.Access.Apps {
			pbApps = append(pbApps, &commonv1.App{
				AppId:        app.AppID,
				Environments: app.Environments,
			})
		}

		pbSnapshots = append(pbSnapshots, &commonv1.ContractSnapshot{
			Name:   snapshot.Name,
			Prefix: snapshot.Prefix,
			Upstream: &commonv1.ContractUpstream{
				Name: snapshot.Upstream.Name,
			},
			AllowUndefinedMethods: snapshot.AllowUndefinedMethods,
			Access: &commonv1.Access{
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
		return grpcerr.Status(h.metricsEnabled, err)
	}

	return nil
}

func (h *ControllerRegistryHandler) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	slog.Debug("Received heartbeat", "controller_id", req.ControllerId)

	var environments []models.EnvironmentInfo
	for _, pbEnv := range req.Environments {
		var bundles []models.BundleInfo
		for _, pbBundle := range pbEnv.Bundles {
			bi := models.BundleInfo{
				Name:       pbBundle.Name,
				Repository: pbBundle.Repository,
				Ref:        pbBundle.Ref,
				Path:       pbBundle.Path,
			}
			if p := pbBundle.GetProvenance(); p != nil {
				bi.ConfigSource = registryConfigSourceString(p.GetSource())
			}
			bundles = append(bundles, bi)
		}
		var services []models.ServiceInfo
		for _, ps := range pbEnv.GetServices() {
			si := models.ServiceInfo{Name: ps.GetName(), Upstream: ps.GetUpstream()}
			if p := ps.GetProvenance(); p != nil {
				si.ConfigSource = registryConfigSourceString(p.GetSource())
			}
			services = append(services, si)
		}
		env := models.EnvironmentInfo{Name: pbEnv.Name, Bundles: bundles, Services: services}
		if pbEnv.GetSourcesFingerprint() != "" {
			env.SourcesFingerprint = pbEnv.GetSourcesFingerprint()
		}
		if pbEnv.EffectiveGeneration != nil {
			g := pbEnv.GetEffectiveGeneration()
			env.EffectiveGeneration = &g
		}
		if s := pbEnv.GetEnvironmentConfigSource(); s != pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED {
			env.EnvironmentConfigSource = registryConfigSourceString(s)
		}
		environments = append(environments, env)
	}

	if err := h.registryUseCase.Heartbeat(ctx, req.ControllerId, environments); err != nil {
		slog.Error("Failed to process heartbeat", "error", err)
		return nil, grpcerr.Status(h.metricsEnabled, err)
	}

	return &pb.HeartbeatResponse{Success: true}, nil
}
