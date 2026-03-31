package handler

// // TenantHandler HTTP handler for tenants
// type TenantHandler struct {
// 	tenantUseCase interfaces.TenantUseCase
// }

// // NewTenantHandler creates a new instance of TenantHandler
// func NewTenantHandler(tenantUseCase interfaces.TenantUseCase) *TenantHandler {
// 	return &TenantHandler{
// 		tenantUseCase: tenantUseCase,
// 	}
// }

// // CreateTenant creates a new tenant
// func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req models.CreateTenantRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err.Error())
// 		return
// 	}

// 	tenant, err := h.tenantUseCase.CreateTenant(r.Context(), &req)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error creating tenant", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusCreated, tenant)
// }

// // GetTenant gets a tenant by ID
// func (h *TenantHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	idStr := getPathParam(r.URL.Path, "/api/v1/tenants/")
// 	id, err := uuid.Parse(idStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid tenant ID", err.Error())
// 		return
// 	}

// 	tenant, err := h.tenantUseCase.GetTenantByID(r.Context(), id)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusNotFound, "Tenant not found", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, tenant)
// }

// // GetTenantByName gets a tenant by name
// func (h *TenantHandler) GetTenantByName(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	name := r.URL.Query().Get("name")
// 	if name == "" {
// 		writeErrorResponse(w, http.StatusBadRequest, "Tenant name is required", "Parameter 'name' cannot be empty")
// 		return
// 	}

// 	tenant, err := h.tenantUseCase.GetTenantByName(r.Context(), name)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusNotFound, "Tenant not found", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, tenant)
// }

// // GetTenants gets a list of all tenants
// func (h *TenantHandler) GetTenants(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	tenants, err := h.tenantUseCase.GetAllTenants(r.Context())
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error getting list of tenants", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, tenants)
// }

// // UpdateTenant updates a tenant
// func (h *TenantHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPut {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	idStr := getPathParam(r.URL.Path, "/api/v1/tenants/")
// 	id, err := uuid.Parse(idStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid tenant ID", err.Error())
// 		return
// 	}

// 	var req models.UpdateTenantRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err.Error())
// 		return
// 	}

// 	tenant, err := h.tenantUseCase.UpdateTenant(r.Context(), id, &req)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error updating tenant", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, tenant)
// }

// // DeleteTenant deletes a tenant
// func (h *TenantHandler) DeleteTenant(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodDelete {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	idStr := getPathParam(r.URL.Path, "/api/v1/tenants/")
// 	id, err := uuid.Parse(idStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid tenant ID", err.Error())
// 		return
// 	}

// 	if err := h.tenantUseCase.DeleteTenant(r.Context(), id); err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error deleting tenant", err.Error())
// 		return
// 	}

// 	writeSuccessMessage(w, http.StatusNoContent, "Tenant successfully deleted")
// }
