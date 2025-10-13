package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Environment represents an environment (dev, preprod, stage, test)
type Environment struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Config    json.RawMessage `json:"config"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Listeners []Listener      `json:"listeners,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CreateEnvironmentRequest request for creating an environment
type CreateEnvironmentRequest struct {
	Name     string          `json:"name" validate:"required,min=1,max=255"`
	Config   json.RawMessage `json:"config" validate:"required"`
	TenantID uuid.UUID       `json:"tenant_id" validate:"required"`
}

// UpdateEnvironmentRequest request for updating an environment
type UpdateEnvironmentRequest struct {
	Name   string          `json:"name" validate:"required,min=1,max=255"`
	Config json.RawMessage `json:"config" validate:"required"`
}

// EnvironmentFilter filter for searching environments
type EnvironmentFilter struct {
	Name     string    `json:"name,omitempty"`
	TenantID uuid.UUID `json:"tenant_id,omitempty"`
}
