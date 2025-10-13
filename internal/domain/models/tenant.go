package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a tenant in the system (dev, prod cluster)
type Tenant struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Environments []Environment `json:"environments,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// CreateTenantRequest request for creating a tenant
type CreateTenantRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// UpdateTenantRequest request for updating a tenant
type UpdateTenantRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// TenantFilter filter for searching tenants
type TenantFilter struct {
	Name string `json:"name,omitempty"`
}
