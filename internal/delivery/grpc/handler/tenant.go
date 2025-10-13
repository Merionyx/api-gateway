package handler

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/delivery/grpc/converter"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	tenantv1 "merionyx/api-gateway/control-plane/pkg/api/tenant/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TenantHandler gRPC handler for tenants
type TenantHandler struct {
	tenantv1.UnimplementedTenantServiceServer
	tenantUseCase interfaces.TenantUseCase
}

// NewTenantHandler creates a new instance of TenantHandler
func NewTenantHandler(tenantUseCase interfaces.TenantUseCase) *TenantHandler {
	return &TenantHandler{
		tenantUseCase: tenantUseCase,
	}
}

// CreateTenant creates a new tenant
func (h *TenantHandler) CreateTenant(ctx context.Context, req *tenantv1.CreateTenantRequest) (*tenantv1.CreateTenantResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be empty")
	}

	domainReq := converter.CreateTenantRequestFromProto(req)

	tenant, err := h.tenantUseCase.CreateTenant(ctx, domainReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &tenantv1.CreateTenantResponse{
		Tenant: converter.TenantToProto(tenant),
	}, nil
}

// GetTenant gets a tenant by ID
func (h *TenantHandler) GetTenant(ctx context.Context, req *tenantv1.GetTenantRequest) (*tenantv1.GetTenantResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant ID")
	}

	tenant, err := h.tenantUseCase.GetTenantByID(ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &tenantv1.GetTenantResponse{
		Tenant: converter.TenantToProto(tenant),
	}, nil
}

// GetTenantByName gets a tenant by name
func (h *TenantHandler) GetTenantByName(ctx context.Context, req *tenantv1.GetTenantByNameRequest) (*tenantv1.GetTenantByNameResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant name is required")
	}

	tenant, err := h.tenantUseCase.GetTenantByName(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &tenantv1.GetTenantByNameResponse{
		Tenant: converter.TenantToProto(tenant),
	}, nil
}

// GetTenants gets a list of all tenants
func (h *TenantHandler) GetTenants(ctx context.Context, req *tenantv1.GetTenantsRequest) (*tenantv1.GetTenantsResponse, error) {
	tenants, err := h.tenantUseCase.GetAllTenants(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &tenantv1.GetTenantsResponse{
		Tenants: converter.TenantsToProto(tenants),
	}, nil
}

// UpdateTenant updates a tenant
func (h *TenantHandler) UpdateTenant(ctx context.Context, req *tenantv1.UpdateTenantRequest) (*tenantv1.UpdateTenantResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant ID")
	}

	domainReq := converter.UpdateTenantRequestFromProto(req)

	tenant, err := h.tenantUseCase.UpdateTenant(ctx, id, domainReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &tenantv1.UpdateTenantResponse{
		Tenant: converter.TenantToProto(tenant),
	}, nil
}

// DeleteTenant deletes a tenant
func (h *TenantHandler) DeleteTenant(ctx context.Context, req *tenantv1.DeleteTenantRequest) (*tenantv1.DeleteTenantResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant ID")
	}

	if err := h.tenantUseCase.DeleteTenant(ctx, id); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &tenantv1.DeleteTenantResponse{}, nil
}
