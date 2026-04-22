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

func provenanceFromPB(p *pb.Provenance) *models.Provenance {
	if p == nil {
		return nil
	}
	if p.GetConfigSource() == pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED {
		return nil
	}
	return &models.Provenance{ConfigSource: registryConfigSourceString(p.GetConfigSource())}
}

func environmentMetaFromPB(m *pb.EnvironmentMeta) *models.EnvironmentMeta {
	if m == nil {
		return nil
	}
	out := &models.EnvironmentMeta{}
	if p := provenanceFromPB(m.GetProvenance()); p != nil {
		out.Provenance = p
	}
	if m.EffectiveGeneration != nil {
		g := *m.EffectiveGeneration
		out.EffectiveGeneration = &g
	}
	if m.SourcesFingerprint != nil {
		out.SourcesFingerprint = *m.SourcesFingerprint
	}
	if out.Provenance == nil && out.EffectiveGeneration == nil && out.SourcesFingerprint == "" {
		return nil
	}
	return out
}

func bundleFromPB(b *pb.BundleInfo) models.BundleInfo {
	bi := models.BundleInfo{
		Name: b.Name, Repository: b.Repository, Ref: b.Ref, Path: b.Path,
	}
	if m := b.GetMeta(); m != nil {
		if p := provenanceFromPB(m.GetProvenance()); p != nil {
			bi.Meta = &models.BundleMeta{Provenance: p}
		}
	}
	return bi
}

func serviceFromPB(s *pb.ServiceInfo) models.ServiceInfo {
	si := models.ServiceInfo{Name: s.Name, Upstream: s.Upstream}
	if m := s.GetMeta(); m != nil {
		if p := provenanceFromPB(m.GetProvenance()); p != nil {
			si.Meta = &models.ServiceMeta{Provenance: p}
		}
	}
	return si
}

func environmentFromPB(e *pb.EnvironmentInfo) models.EnvironmentInfo {
	var bundles []models.BundleInfo
	for _, b := range e.Bundles {
		bundles = append(bundles, bundleFromPB(b))
	}
	var services []models.ServiceInfo
	for _, s := range e.Services {
		services = append(services, serviceFromPB(s))
	}
	return models.EnvironmentInfo{
		Name:     e.Name,
		Bundles:  bundles,
		Services: services,
		Meta:     environmentMetaFromPB(e.GetMeta()),
	}
}

func (h *ControllerRegistryHandler) RegisterController(ctx context.Context, req *pb.RegisterControllerRequest) (*pb.RegisterControllerResponse, error) {
	slog.Info("Received register controller request", "controller_id", req.ControllerId, "tenant", req.Tenant)

	environments := make([]models.EnvironmentInfo, 0, len(req.Environments))
	for _, pbEnv := range req.Environments {
		environments = append(environments, environmentFromPB(pbEnv))
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

	environments := make([]models.EnvironmentInfo, 0, len(req.Environments))
	for _, pbEnv := range req.Environments {
		environments = append(environments, environmentFromPB(pbEnv))
	}

	if err := h.registryUseCase.Heartbeat(ctx, req.ControllerId, environments); err != nil {
		slog.Error("Failed to process heartbeat", "error", err)
		return nil, grpcerr.Status(h.metricsEnabled, err)
	}

	return &pb.HeartbeatResponse{Success: true}, nil
}
