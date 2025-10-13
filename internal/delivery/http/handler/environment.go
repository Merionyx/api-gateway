package handler

import (
	"encoding/json"
	"net/http"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"

	"github.com/google/uuid"
)

// EnvironmentHandler HTTP handler for environments
type EnvironmentHandler struct {
	environmentUseCase interfaces.EnvironmentUseCase
}

// NewEnvironmentHandler creates a new instance of EnvironmentHandler
func NewEnvironmentHandler(environmentUseCase interfaces.EnvironmentUseCase) *EnvironmentHandler {
	return &EnvironmentHandler{
		environmentUseCase: environmentUseCase,
	}
}

// CreateEnvironment creates a new environment
func (h *EnvironmentHandler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err.Error())
		return
	}

	environment, err := h.environmentUseCase.CreateEnvironment(r.Context(), &req)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error creating environment", err.Error())
		return
	}

	writeSuccessResponse(w, http.StatusCreated, environment)
}

// GetEnvironment gets an environment by ID
func (h *EnvironmentHandler) GetEnvironment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := getPathParam(r.URL.Path, "/api/v1/environments/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
		return
	}

	environment, err := h.environmentUseCase.GetEnvironmentByID(r.Context(), id)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Environment not found", err.Error())
		return
	}

	writeSuccessResponse(w, http.StatusOK, environment)
}

// GetEnvironmentByName gets an environment by name
func (h *EnvironmentHandler) GetEnvironmentByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Environment name is required", "Parameter 'name' cannot be empty")
		return
	}

	environment, err := h.environmentUseCase.GetEnvironmentByName(r.Context(), name)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "Environment not found", err.Error())
		return
	}

	writeSuccessResponse(w, http.StatusOK, environment)
}

// GetEnvironments gets a list of all environments
func (h *EnvironmentHandler) GetEnvironments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	environments, err := h.environmentUseCase.GetAllEnvironments(r.Context())
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error getting list of environments", err.Error())
		return
	}

	writeSuccessResponse(w, http.StatusOK, environments)
}

// GetEnvironmentsByTenant gets environments by tenant
func (h *EnvironmentHandler) GetEnvironmentsByTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantIDStr := getPathParam(r.URL.Path, "/api/v1/tenants/")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid tenant ID", err.Error())
		return
	}

	environments, err := h.environmentUseCase.GetEnvironmentsByTenantID(r.Context(), tenantID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error getting environments by tenant", err.Error())
		return
	}

	writeSuccessResponse(w, http.StatusOK, environments)
}

// UpdateEnvironment updates an environment
func (h *EnvironmentHandler) UpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := getPathParam(r.URL.Path, "/api/v1/environments/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
		return
	}

	var req models.UpdateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err.Error())
		return
	}

	environment, err := h.environmentUseCase.UpdateEnvironment(r.Context(), id, &req)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error updating environment", err.Error())
		return
	}

	writeSuccessResponse(w, http.StatusOK, environment)
}

// DeleteEnvironment deletes an environment
func (h *EnvironmentHandler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := getPathParam(r.URL.Path, "/api/v1/environments/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
		return
	}

	if err := h.environmentUseCase.DeleteEnvironment(r.Context(), id); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error deleting environment", err.Error())
		return
	}

	writeSuccessMessage(w, http.StatusNoContent, "Environment successfully deleted")
}

// MapEnvironmentToTenant maps an environment to a tenant
func (h *EnvironmentHandler) MapEnvironmentToTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from the URL path
	envIDStr := getPathParam(r.URL.Path, "/api/v1/environments/")
	envID, err := uuid.Parse(envIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
		return
	}

	tenantIDStr := getPathParamAfter(r.URL.Path, "/tenants/")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid tenant ID", err.Error())
		return
	}

	if err := h.environmentUseCase.MapEnvironmentToTenant(r.Context(), envID, tenantID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error mapping environment to tenant", err.Error())
		return
	}

	writeSuccessMessage(w, http.StatusOK, "Environment successfully mapped to tenant")
}

// UnmapEnvironmentFromTenant unmaps an environment from a tenant
func (h *EnvironmentHandler) UnmapEnvironmentFromTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from the URL path
	envIDStr := getPathParam(r.URL.Path, "/api/v1/environments/")
	envID, err := uuid.Parse(envIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
		return
	}

	tenantIDStr := getPathParamAfter(r.URL.Path, "/tenants/")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid tenant ID", err.Error())
		return
	}

	if err := h.environmentUseCase.UnmapEnvironmentFromTenant(r.Context(), envID, tenantID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Error unmapping environment from tenant", err.Error())
		return
	}

	writeSuccessMessage(w, http.StatusOK, "Environment successfully unmapped from tenant")
}
