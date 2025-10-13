package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Listener represents a listener Envoy
type Listener struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Config        json.RawMessage `json:"config"`
	EnvironmentID uuid.UUID       `json:"environment_id"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// CreateListenerRequest request for creating a listener
type CreateListenerRequest struct {
	Name          string          `json:"name" validate:"required,min=1,max=255"`
	Config        json.RawMessage `json:"config" validate:"required"`
	EnvironmentID uuid.UUID       `json:"environment_id" validate:"required"`
}

// UpdateListenerRequest request for updating a listener
type UpdateListenerRequest struct {
	Name   string          `json:"name" validate:"required,min=1,max=255"`
	Config json.RawMessage `json:"config" validate:"required"`
}

// ListenerFilter filter for searching listeners
type ListenerFilter struct {
	Name          string    `json:"name,omitempty"`
	EnvironmentID uuid.UUID `json:"environment_id,omitempty"`
}
