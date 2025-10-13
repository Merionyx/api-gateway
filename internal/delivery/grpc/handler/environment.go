package handler

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/delivery/grpc/converter"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	environmentv1 "merionyx/api-gateway/control-plane/pkg/api/environment/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnvironmentHandler gRPC handler for environments
type EnvironmentHandler struct {
	environmentv1.UnimplementedEnvironmentServiceServer
	environmentUseCase interfaces.EnvironmentUseCase
}

// NewEnvironmentHandler creates a new instance of EnvironmentHandler
func NewEnvironmentHandler(environmentUseCase interfaces.EnvironmentUseCase) *EnvironmentHandler {
	return &EnvironmentHandler{
		environmentUseCase: environmentUseCase,
	}
}

// CreateEnvironment creates a new environment
func (h *EnvironmentHandler) CreateEnvironment(ctx context.Context, req *environmentv1.CreateEnvironmentRequest) (*environmentv1.CreateEnvironmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be empty")
	}

	domainReq, err := converter.CreateEnvironmentRequestFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	environment, err := h.environmentUseCase.CreateEnvironment(ctx, domainReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoEnv, err := converter.EnvironmentToProto(environment)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.CreateEnvironmentResponse{
		Environment: protoEnv,
	}, nil
}

// GetEnvironment gets an environment by ID
func (h *EnvironmentHandler) GetEnvironment(ctx context.Context, req *environmentv1.GetEnvironmentRequest) (*environmentv1.GetEnvironmentResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "environment ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	environment, err := h.environmentUseCase.GetEnvironmentByID(ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	protoEnv, err := converter.EnvironmentToProto(environment)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.GetEnvironmentResponse{
		Environment: protoEnv,
	}, nil
}

// GetEnvironmentByName gets an environment by name
func (h *EnvironmentHandler) GetEnvironmentByName(ctx context.Context, req *environmentv1.GetEnvironmentByNameRequest) (*environmentv1.GetEnvironmentByNameResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "environment name is required")
	}

	environment, err := h.environmentUseCase.GetEnvironmentByName(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	protoEnv, err := converter.EnvironmentToProto(environment)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.GetEnvironmentByNameResponse{
		Environment: protoEnv,
	}, nil
}

// GetEnvironments gets a list of all environments
func (h *EnvironmentHandler) GetEnvironments(ctx context.Context, req *environmentv1.GetEnvironmentsRequest) (*environmentv1.GetEnvironmentsResponse, error) {
	environments, err := h.environmentUseCase.GetAllEnvironments(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoEnvs, err := converter.EnvironmentsToProto(environments)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.GetEnvironmentsResponse{
		Environments: protoEnvs,
	}, nil
}

// GetEnvironmentsByTenant gets environments by tenant
func (h *EnvironmentHandler) GetEnvironmentsByTenant(ctx context.Context, req *environmentv1.GetEnvironmentsByTenantRequest) (*environmentv1.GetEnvironmentsByTenantResponse, error) {
	if req == nil || req.TenantId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant ID is required")
	}

	tenantID, err := uuid.Parse(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant ID")
	}

	environments, err := h.environmentUseCase.GetEnvironmentsByTenantID(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoEnvs, err := converter.EnvironmentsToProto(environments)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.GetEnvironmentsByTenantResponse{
		Environments: protoEnvs,
	}, nil
}

// UpdateEnvironment updates an environment
func (h *EnvironmentHandler) UpdateEnvironment(ctx context.Context, req *environmentv1.UpdateEnvironmentRequest) (*environmentv1.UpdateEnvironmentResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "environment ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	domainReq, err := converter.UpdateEnvironmentRequestFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	environment, err := h.environmentUseCase.UpdateEnvironment(ctx, id, domainReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoEnv, err := converter.EnvironmentToProto(environment)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.UpdateEnvironmentResponse{
		Environment: protoEnv,
	}, nil
}

// DeleteEnvironment deletes an environment
func (h *EnvironmentHandler) DeleteEnvironment(ctx context.Context, req *environmentv1.DeleteEnvironmentRequest) (*environmentv1.DeleteEnvironmentResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "environment ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	if err := h.environmentUseCase.DeleteEnvironment(ctx, id); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.DeleteEnvironmentResponse{}, nil
}

// MapEnvironmentToTenant maps an environment to a tenant
func (h *EnvironmentHandler) MapEnvironmentToTenant(ctx context.Context, req *environmentv1.MapEnvironmentToTenantRequest) (*environmentv1.MapEnvironmentToTenantResponse, error) {
	if req == nil || req.EnvironmentId == "" || req.TenantId == "" {
		return nil, status.Error(codes.InvalidArgument, "environment ID and tenant ID are required")
	}

	environmentID, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	tenantID, err := uuid.Parse(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant ID")
	}

	if err := h.environmentUseCase.MapEnvironmentToTenant(ctx, environmentID, tenantID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.MapEnvironmentToTenantResponse{}, nil
}

// UnmapEnvironmentFromTenant unmaps an environment from a tenant
func (h *EnvironmentHandler) UnmapEnvironmentFromTenant(ctx context.Context, req *environmentv1.UnmapEnvironmentFromTenantRequest) (*environmentv1.UnmapEnvironmentFromTenantResponse, error) {
	if req == nil || req.EnvironmentId == "" || req.TenantId == "" {
		return nil, status.Error(codes.InvalidArgument, "environment ID and tenant ID are required")
	}

	environmentID, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	tenantID, err := uuid.Parse(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant ID")
	}

	if err := h.environmentUseCase.UnmapEnvironmentFromTenant(ctx, environmentID, tenantID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &environmentv1.UnmapEnvironmentFromTenantResponse{}, nil
}
