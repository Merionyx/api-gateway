package handler

import (
	"context"
	"time"

	commonv1 "github.com/merionyx/api-gateway/pkg/grpc/common/v1"
	environmentsv1 "github.com/merionyx/api-gateway/pkg/grpc/environments/v1"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/shared/election"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EnvironmentsHandler struct {
	environmentsv1.UnimplementedEnvironmentsServiceServer
	environmentsUseCase interfaces.EnvironmentsUseCase
	leader              election.LeaderGate
	bundleIndex         *bundleenv.Index
	metricsEnabled      bool
}

func NewEnvironmentsHandler(
	environmentsUseCase interfaces.EnvironmentsUseCase,
	leader election.LeaderGate,
	bundleIndex *bundleenv.Index,
	metricsEnabled bool,
) *EnvironmentsHandler {
	if leader == nil {
		leader = election.NoopGate{}
	}
	return &EnvironmentsHandler{
		environmentsUseCase: environmentsUseCase,
		leader:              leader,
		bundleIndex:         bundleIndex,
		metricsEnabled:      metricsEnabled,
	}
}

func (h *EnvironmentsHandler) rebuildBundleIndexAsync() {
	if h.bundleIndex == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		h.bundleIndex.Rebuild(ctx)
		ctrlmetrics.RecordBundleEnvIndexRebuild(h.metricsEnabled)
	}()
}

func (h *EnvironmentsHandler) CreateEnvironment(ctx context.Context, req *environmentsv1.CreateEnvironmentRequest) (*environmentsv1.CreateEnvironmentResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "CreateEnvironment")
	defer span.End()

	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !h.leader.IsLeader() {
		return nil, status.Error(codes.FailedPrecondition, "not the etcd write leader; retry another Gateway Controller replica or wait for leadership")
	}

	env, err := h.environmentsUseCase.CreateEnvironment(ctx, &models.CreateEnvironmentRequest{
		Name:     req.Name,
		Type:     "manual",
		Bundles:  protoToModelBundlesConfig(req.Bundles),
		Services: protoToModelServicesConfig(req.Services),
	})
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	h.rebuildBundleIndexAsync()

	return &environmentsv1.CreateEnvironmentResponse{
		Environment: modelToProtoEnvironment(env),
	}, nil
}

func (h *EnvironmentsHandler) GetEnvironment(ctx context.Context, req *environmentsv1.GetEnvironmentRequest) (*environmentsv1.GetEnvironmentResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "GetEnvironment")
	defer span.End()

	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	env, err := h.environmentsUseCase.GetEnvironment(ctx, req.Name)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &environmentsv1.GetEnvironmentResponse{
		Environment: modelToProtoEnvironment(env),
	}, nil
}

func (h *EnvironmentsHandler) ListEnvironments(ctx context.Context, req *environmentsv1.ListEnvironmentsRequest) (*environmentsv1.ListEnvironmentsResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "ListEnvironments")
	defer span.End()

	environments, err := h.environmentsUseCase.ListEnvironments(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var protoEnvs []*environmentsv1.Environment
	for _, env := range environments {
		protoEnvs = append(protoEnvs, modelToProtoEnvironment(env))
	}

	return &environmentsv1.ListEnvironmentsResponse{
		Environments: protoEnvs,
	}, nil
}

func (h *EnvironmentsHandler) UpdateEnvironment(ctx context.Context, req *environmentsv1.UpdateEnvironmentRequest) (*environmentsv1.UpdateEnvironmentResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "UpdateEnvironment")
	defer span.End()

	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !h.leader.IsLeader() {
		return nil, status.Error(codes.FailedPrecondition, "not the etcd write leader; retry another Gateway Controller replica or wait for leadership")
	}

	env, err := h.environmentsUseCase.UpdateEnvironment(ctx, &models.UpdateEnvironmentRequest{
		Name:     req.Name,
		Bundles:  protoToModelBundlesConfig(req.Bundles),
		Services: protoToModelServicesConfig(req.Services),
	})
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	h.rebuildBundleIndexAsync()

	return &environmentsv1.UpdateEnvironmentResponse{
		Environment: modelToProtoEnvironment(env),
	}, nil
}

func (h *EnvironmentsHandler) DeleteEnvironment(ctx context.Context, req *environmentsv1.DeleteEnvironmentRequest) (*environmentsv1.DeleteEnvironmentResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanHandlerPkg, "DeleteEnvironment")
	defer span.End()

	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !h.leader.IsLeader() {
		return nil, status.Error(codes.FailedPrecondition, "not the etcd write leader; retry another Gateway Controller replica or wait for leadership")
	}

	if err := h.environmentsUseCase.DeleteEnvironment(ctx, req.Name); err != nil {
		telemetry.MarkError(span, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	h.rebuildBundleIndexAsync()

	return &environmentsv1.DeleteEnvironmentResponse{
		Success: true,
	}, nil
}

// Helper functions for conversion between proto and model

func modelToProtoEnvironment(env *models.Environment) *environmentsv1.Environment {
	return &environmentsv1.Environment{
		Name:      env.Name,
		Type:      env.Type,
		Bundles:   modelToProtoBundlesConfig(env.Bundles),
		Services:  modelToProtoServicesConfig(env.Services),
		Snapshots: modelToProtoSnapshots(env.Snapshots),
	}
}

func modelToProtoBundlesConfig(bundles *models.EnvironmentBundleConfig) *environmentsv1.EnvironmentBundleConfig {
	if bundles == nil {
		return nil
	}

	var static []*environmentsv1.StaticBundleConfig
	for _, c := range bundles.Static {
		static = append(static, &environmentsv1.StaticBundleConfig{
			Name:       c.Name,
			Repository: c.Repository,
			Ref:        c.Ref,
			Path:       c.Path,
		})
	}

	return &environmentsv1.EnvironmentBundleConfig{
		Static: static,
	}
}

func modelToProtoServicesConfig(services *models.EnvironmentServiceConfig) *environmentsv1.EnvironmentServiceConfig {
	if services == nil {
		return nil
	}

	var static []*environmentsv1.StaticServiceConfig
	for _, s := range services.Static {
		static = append(static, &environmentsv1.StaticServiceConfig{
			Name:     s.Name,
			Upstream: s.Upstream,
		})
	}

	return &environmentsv1.EnvironmentServiceConfig{
		Static: static,
	}
}

func modelToProtoSnapshots(snapshots []models.ContractSnapshot) []*commonv1.ContractSnapshot {
	var result []*commonv1.ContractSnapshot
	for _, s := range snapshots {
		var pbApps []*commonv1.App
		for _, app := range s.Access.Apps {
			pbApps = append(pbApps, &commonv1.App{
				AppId:        app.AppID,
				Environments: app.Environments,
			})
		}
		result = append(result, &commonv1.ContractSnapshot{
			Name:                  s.Name,
			Prefix:                s.Prefix,
			Upstream:              &commonv1.ContractUpstream{Name: s.Upstream.Name},
			AllowUndefinedMethods: s.AllowUndefinedMethods,
			Access: &commonv1.Access{
				Secure: s.Access.Secure,
				Apps:   pbApps,
			},
		})
	}
	return result
}

func protoToModelBundlesConfig(bundles *environmentsv1.EnvironmentBundleConfig) *models.EnvironmentBundleConfig {
	if bundles == nil {
		return &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{}}
	}

	var static []models.StaticContractBundleConfig
	for _, c := range bundles.Static {
		static = append(static, models.StaticContractBundleConfig{
			Name:       c.Name,
			Repository: c.Repository,
			Ref:        c.Ref,
			Path:       c.Path,
		})
	}

	return &models.EnvironmentBundleConfig{Static: static}
}

func protoToModelServicesConfig(services *environmentsv1.EnvironmentServiceConfig) *models.EnvironmentServiceConfig {
	if services == nil {
		return &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{}}
	}

	var static []models.StaticServiceConfig
	for _, s := range services.Static {
		static = append(static, models.StaticServiceConfig{
			Name:     s.Name,
			Upstream: s.Upstream,
		})
	}

	return &models.EnvironmentServiceConfig{Static: static}
}
