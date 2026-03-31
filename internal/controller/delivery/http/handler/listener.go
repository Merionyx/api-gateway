package handler

// ListenerHandler HTTP handler for listeners
type ListenerHandler struct {
}

// NewListenerHandler creates a new instance of ListenerHandler
func NewListenerHandler() *ListenerHandler {
	return &ListenerHandler{}
}

// // CreateListener creates a new listener
// func (h *ListenerHandler) CreateListener(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	var req models.CreateListenerRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err.Error())
// 		return
// 	}

// 	listener, err := h.listenerUseCase.CreateListener(r.Context(), &req)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error creating listener", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusCreated, listener)
// }

// // GetListener gets a listener by ID
// func (h *ListenerHandler) GetListener(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	idStr := getPathParam(r.URL.Path, "/api/v1/listeners/")
// 	id, err := uuid.Parse(idStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid listener ID", err.Error())
// 		return
// 	}

// 	listener, err := h.listenerUseCase.GetListenerByID(r.Context(), id)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusNotFound, "Listener not found", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, listener)
// }

// // GetListenerByName gets a listener by name
// func (h *ListenerHandler) GetListenerByName(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	name := r.URL.Query().Get("name")
// 	if name == "" {
// 		writeErrorResponse(w, http.StatusBadRequest, "Listener name is required", "Parameter 'name' cannot be empty")
// 		return
// 	}

// 	listener, err := h.listenerUseCase.GetListenerByName(r.Context(), name)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusNotFound, "Listener not found", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, listener)
// }

// // GetListeners gets a list of all listeners
// func (h *ListenerHandler) GetListeners(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	listeners, err := h.listenerUseCase.GetAllListeners(r.Context())
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error getting list of listeners", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, listeners)
// }

// // GetListenersByEnvironment gets listeners by environment
// func (h *ListenerHandler) GetListenersByEnvironment(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	envIDStr := getPathParam(r.URL.Path, "/api/v1/environments/")
// 	envID, err := uuid.Parse(envIDStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
// 		return
// 	}

// 	listeners, err := h.listenerUseCase.GetListenersByEnvironmentID(r.Context(), envID)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error getting listeners by environment", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, listeners)
// }

// // UpdateListener updates a listener
// func (h *ListenerHandler) UpdateListener(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPut {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	idStr := getPathParam(r.URL.Path, "/api/v1/listeners/")
// 	id, err := uuid.Parse(idStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid listener ID", err.Error())
// 		return
// 	}

// 	var req models.UpdateListenerRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid request data", err.Error())
// 		return
// 	}

// 	listener, err := h.listenerUseCase.UpdateListener(r.Context(), id, &req)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error updating listener", err.Error())
// 		return
// 	}

// 	writeSuccessResponse(w, http.StatusOK, listener)
// }

// // DeleteListener deletes a listener
// func (h *ListenerHandler) DeleteListener(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodDelete {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	idStr := getPathParam(r.URL.Path, "/api/v1/listeners/")
// 	id, err := uuid.Parse(idStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid listener ID", err.Error())
// 		return
// 	}

// 	if err := h.listenerUseCase.DeleteListener(r.Context(), id); err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error deleting listener", err.Error())
// 		return
// 	}

// 	writeSuccessMessage(w, http.StatusNoContent, "Listener successfully deleted")
// }

// // MapListenerToEnvironment maps a listener to an environment
// func (h *ListenerHandler) MapListenerToEnvironment(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// Extract ID from the URL path
// 	listenerIDStr := getPathParam(r.URL.Path, "/api/v1/listeners/")
// 	listenerID, err := uuid.Parse(listenerIDStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid listener ID", err.Error())
// 		return
// 	}

// 	envIDStr := getPathParamAfter(r.URL.Path, "/environments/")
// 	envID, err := uuid.Parse(envIDStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
// 		return
// 	}

// 	if err := h.listenerUseCase.MapListenerToEnvironment(r.Context(), listenerID, envID); err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error mapping listener to environment", err.Error())
// 		return
// 	}

// 	writeSuccessMessage(w, http.StatusOK, "Listener successfully mapped to environment")
// }

// // UnmapListenerFromEnvironment unmaps a listener from an environment
// func (h *ListenerHandler) UnmapListenerFromEnvironment(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodDelete {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// Extract ID from the URL path
// 	listenerIDStr := getPathParam(r.URL.Path, "/api/v1/listeners/")
// 	listenerID, err := uuid.Parse(listenerIDStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid listener ID", err.Error())
// 		return
// 	}

// 	envIDStr := getPathParamAfter(r.URL.Path, "/environments/")
// 	envID, err := uuid.Parse(envIDStr)
// 	if err != nil {
// 		writeErrorResponse(w, http.StatusBadRequest, "Invalid environment ID", err.Error())
// 		return
// 	}

// 	if err := h.listenerUseCase.UnmapListenerFromEnvironment(r.Context(), listenerID, envID); err != nil {
// 		writeErrorResponse(w, http.StatusInternalServerError, "Error unmapping listener from environment", err.Error())
// 		return
// 	}

// 	writeSuccessMessage(w, http.StatusOK, "Listener successfully unmapped from environment")
// }
