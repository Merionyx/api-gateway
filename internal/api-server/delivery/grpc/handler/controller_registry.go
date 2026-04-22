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
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// spanHandlerPkg is the import path of this package (span names: {importPath}.MethodName).
const spanHandlerPkg = "github.com/merionyx/api-gateway/internal/api-server/delivery/grpc/handler"

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
	cs := registryConfigSourceString(p.GetConfigSource())
	if p.GetConfigSource() == pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED {
		cs = ""
	}
	ld := p.GetLayerDetail()
	if cs == "" && ld == "" {
		return nil
	}
	return &models.Provenance{ConfigSource: cs, LayerDetail: ld}
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
	if v := m.GetEnvironmentType(); v != "" {
		out.EnvironmentType = v
	}
	if v := m.GetMaterializedUpdatedAt(); v != "" {
		out.MaterializedUpdatedAt = v
	}
	if m.MaterializedSchemaVersion != nil {
		sv := *m.MaterializedSchemaVersion
		out.MaterializedSchemaVersion = &sv
	}
	if m.MaterializedMismatch != nil {
		mm := *m.MaterializedMismatch
		out.MaterializedMismatch = &mm
	}
	if out.Provenance == nil && out.EffectiveGeneration == nil && out.SourcesFingerprint == "" &&
		out.EnvironmentType == "" && out.MaterializedUpdatedAt == "" && out.MaterializedSchemaVersion == nil && out.MaterializedMismatch == nil {
		return nil
	}
	return out
}

func bundleFromPB(b *pb.BundleInfo) models.BundleInfo {
	bi := models.BundleInfo{
		Name: b.Name, Repository: b.Repository, Ref: b.Ref, Path: b.Path,
	}
	if m := b.GetMeta(); m != nil {
		if p := provenanceFromPB(m.GetProvenance()); p != nil ||
			m.GetResolvedRef() != "" || m.GetLastSyncUtc() != "" || m.GetSyncError() != "" || m.GetK8SResourceRef() != "" {
			bm := &models.BundleMeta{}
			if p != nil {
				bm.Provenance = p
			}
			if v := m.GetResolvedRef(); v != "" {
				bm.ResolvedRef = v
			}
			if v := m.GetLastSyncUtc(); v != "" {
				bm.LastSyncUTC = v
			}
			if v := m.GetSyncError(); v != "" {
				bm.SyncError = v
			}
			if v := m.GetK8SResourceRef(); v != "" {
				bm.K8SResourceRef = v
			}
			if bm.Provenance != nil || bm.ResolvedRef != "" || bm.LastSyncUTC != "" || bm.SyncError != "" || bm.K8SResourceRef != "" {
				bi.Meta = bm
			}
		}
	}
	return bi
}

func serviceLineScopeFromPB(s pb.ServiceLineScope) string {
	switch s {
	case pb.ServiceLineScope_SERVICE_LINE_SCOPE_ENVIRONMENT:
		return "environment"
	case pb.ServiceLineScope_SERVICE_LINE_SCOPE_CONTROLLER_ROOT:
		return "controller_root"
	default:
		return ""
	}
}

func serviceFromPB(s *pb.ServiceInfo) models.ServiceInfo {
	si := models.ServiceInfo{Name: s.Name, Upstream: s.Upstream, Scope: serviceLineScopeFromPB(s.GetScope())}
	if m := s.GetMeta(); m != nil {
		if p := provenanceFromPB(m.GetProvenance()); p != nil || m.GetK8SServiceRef() != "" {
			sm := &models.ServiceMeta{}
			if p != nil {
				sm.Provenance = p
			}
			if v := m.GetK8SServiceRef(); v != "" {
				sm.K8sServiceRef = v
			}
			if sm.Provenance != nil || sm.K8sServiceRef != "" {
				si.Meta = sm
			}
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
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "RegisterController")
	defer span.End()

	slog.Info("Received register controller request", "controller_id", req.ControllerId, "tenant", req.Tenant)

	environments := make([]models.EnvironmentInfo, 0, len(req.Environments))
	for _, pbEnv := range req.Environments {
		environments = append(environments, environmentFromPB(pbEnv))
	}

	info := models.ControllerInfo{
		ControllerID:           req.ControllerId,
		Tenant:                 req.Tenant,
		Environments:           environments,
		RegistryPayloadVersion: req.RegistryPayloadVersion,
	}

	if err := h.registryUseCase.RegisterController(ctx, info); err != nil {
		telemetry.MarkError(span, err)
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
	ctx, span := telemetry.ServerSpan(stream.Context(), spanHandlerPkg, "StreamSnapshots")
	defer span.End()

	slog.Info("Starting snapshot stream", "controller_id", req.ControllerId)

	wrapper := &snapshotStreamWrapper{stream: stream}

	if err := h.registryUseCase.StreamSnapshots(ctx, req.ControllerId, wrapper); err != nil {
		telemetry.MarkError(span, err)
		slog.Error("Stream error", "error", err)
		return grpcerr.Status(h.metricsEnabled, err)
	}

	return nil
}

func (h *ControllerRegistryHandler) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "Heartbeat")
	defer span.End()

	slog.Debug("Received heartbeat", "controller_id", req.ControllerId)

	environments := make([]models.EnvironmentInfo, 0, len(req.Environments))
	for _, pbEnv := range req.Environments {
		environments = append(environments, environmentFromPB(pbEnv))
	}

	if err := h.registryUseCase.Heartbeat(ctx, req.ControllerId, environments, req.RegistryPayloadVersion); err != nil {
		telemetry.MarkError(span, err)
		slog.Error("Failed to process heartbeat", "error", err)
		return nil, grpcerr.Status(h.metricsEnabled, err)
	}

	return &pb.HeartbeatResponse{Success: true}, nil
}
