package handler

import (
	"context"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/controller/repository/git"
	environmentsv1 "merionyx/api-gateway/pkg/api/environments/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EnvironmentsHandler struct {
	environmentsv1.UnimplementedEnvironmentsServiceServer
	environmentsUseCase interfaces.EnvironmentsUseCase
}

func NewEnvironmentsHandler(environmentsUseCase interfaces.EnvironmentsUseCase) *EnvironmentsHandler {
	return &EnvironmentsHandler{environmentsUseCase: environmentsUseCase}
}

func (h *EnvironmentsHandler) CreateEnvironment(ctx context.Context, req *environmentsv1.CreateEnvironmentRequest) (*environmentsv1.CreateEnvironmentResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	env, err := h.environmentsUseCase.CreateEnvironment(ctx, &models.CreateEnvironmentRequest{
		Name:     req.Name,
		Type:     "manual",
		Bundles:  protoToModelBundlesConfig(req.Bundles),
		Services: protoToModelServicesConfig(req.Services),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentsv1.CreateEnvironmentResponse{
		Environment: modelToProtoEnvironment(env),
	}, nil
}

func (h *EnvironmentsHandler) GetEnvironment(ctx context.Context, req *environmentsv1.GetEnvironmentRequest) (*environmentsv1.GetEnvironmentResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	env, err := h.environmentsUseCase.GetEnvironment(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &environmentsv1.GetEnvironmentResponse{
		Environment: modelToProtoEnvironment(env),
	}, nil
}

func (h *EnvironmentsHandler) ListEnvironments(ctx context.Context, req *environmentsv1.ListEnvironmentsRequest) (*environmentsv1.ListEnvironmentsResponse, error) {
	environments, err := h.environmentsUseCase.ListEnvironments(ctx)
	if err != nil {
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
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	env, err := h.environmentsUseCase.UpdateEnvironment(ctx, &models.UpdateEnvironmentRequest{
		Name:     req.Name,
		Bundles:  protoToModelBundlesConfig(req.Bundles),
		Services: protoToModelServicesConfig(req.Services),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentsv1.UpdateEnvironmentResponse{
		Environment: modelToProtoEnvironment(env),
	}, nil
}

func (h *EnvironmentsHandler) DeleteEnvironment(ctx context.Context, req *environmentsv1.DeleteEnvironmentRequest) (*environmentsv1.DeleteEnvironmentResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if err := h.environmentsUseCase.DeleteEnvironment(ctx, req.Name); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentsv1.DeleteEnvironmentResponse{
		Success: true,
	}, nil
}

// Helper functions для конвертации между proto и model

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

func modelToProtoSnapshots(snapshots []git.ContractSnapshot) []*environmentsv1.ContractSnapshot {
	var result []*environmentsv1.ContractSnapshot
	for _, s := range snapshots {
		result = append(result, &environmentsv1.ContractSnapshot{
			Name:                  s.Name,
			Prefix:                s.Prefix,
			Upstream:              &environmentsv1.ContractUpstream{Name: s.Upstream.Name},
			AllowUndefinedMethods: s.AllowUndefinedMethods,
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
