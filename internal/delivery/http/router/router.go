package router

import (
	"encoding/json"
	"net/http"
	"strings"

	"merionyx/api-gateway/control-plane/internal/delivery/http/handler"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
)

// Router structure for setting up routes
type Router struct {
	tenantHandler      *handler.TenantHandler
	environmentHandler *handler.EnvironmentHandler
	listenerHandler    *handler.ListenerHandler
	mux                *http.ServeMux
}

// NewRouter creates a new instance of Router
func NewRouter(
	tenantUseCase interfaces.TenantUseCase,
	environmentUseCase interfaces.EnvironmentUseCase,
	listenerUseCase interfaces.ListenerUseCase,
) *Router {
	return &Router{
		tenantHandler:      handler.NewTenantHandler(tenantUseCase),
		environmentHandler: handler.NewEnvironmentHandler(environmentUseCase),
		listenerHandler:    handler.NewListenerHandler(listenerUseCase),
		mux:                http.NewServeMux(),
	}
}

// SetupRoutes sets up all routes
func (r *Router) SetupRoutes() http.Handler {
	// Health check
	r.mux.HandleFunc("/health", r.healthCheck)

	// API v1 routes - use one handler for all routes
	r.mux.HandleFunc("/api/v1/", r.handleAPIRoutes)

	// Add CORS middleware
	return r.corsMiddleware(r.mux)
}

// healthCheck handler for checking the health of the service
func (r *Router) healthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "api-gateway-control-plane",
	})
}

// handleAPIRoutes main handler for all API routes
func (r *Router) handleAPIRoutes(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	switch {
	// Tenants routes
	case path == "/api/v1/tenants":
		r.handleTenants(w, req)
	case path == "/api/v1/tenants/by-name":
		r.tenantHandler.GetTenantByName(w, req)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.Count(path, "/") == 4:
		// /api/v1/tenants/{id}
		r.handleTenantByID(w, req)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.Contains(path, "/environments"):
		// /api/v1/tenants/{tenant_id}/environments
		r.environmentHandler.GetEnvironmentsByTenant(w, req)

	// Environments routes
	case path == "/api/v1/environments":
		r.handleEnvironments(w, req)
	case path == "/api/v1/environments/by-name":
		r.environmentHandler.GetEnvironmentByName(w, req)
	case strings.HasPrefix(path, "/api/v1/environments/") && strings.Count(path, "/") == 4:
		// /api/v1/environments/{id}
		r.handleEnvironmentByID(w, req)
	case strings.HasPrefix(path, "/api/v1/environments/") && strings.Contains(path, "/listeners"):
		// /api/v1/environments/{environment_id}/listeners
		r.listenerHandler.GetListenersByEnvironment(w, req)
	case strings.HasPrefix(path, "/api/v1/environments/") && strings.Contains(path, "/tenants/"):
		// /api/v1/environments/{id}/tenants/{tenant_id}
		r.handleEnvironmentTenantMapping(w, req)

	// Listeners routes
	case path == "/api/v1/listeners":
		r.handleListeners(w, req)
	case path == "/api/v1/listeners/by-name":
		r.listenerHandler.GetListenerByName(w, req)
	case strings.HasPrefix(path, "/api/v1/listeners/") && strings.Count(path, "/") == 4:
		// /api/v1/listeners/{id}
		r.handleListenerByID(w, req)
	case strings.HasPrefix(path, "/api/v1/listeners/") && strings.Contains(path, "/environments/"):
		// /api/v1/listeners/{id}/environments/{environment_id}
		r.handleListenerEnvironmentMapping(w, req)

	default:
		http.NotFound(w, req)
	}
}

// handleTenants handles requests to /api/v1/tenants
func (r *Router) handleTenants(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.tenantHandler.GetTenants(w, req)
	case http.MethodPost:
		r.tenantHandler.CreateTenant(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTenantByID handles requests to /api/v1/tenants/{id}
func (r *Router) handleTenantByID(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.tenantHandler.GetTenant(w, req)
	case http.MethodPut:
		r.tenantHandler.UpdateTenant(w, req)
	case http.MethodDelete:
		r.tenantHandler.DeleteTenant(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEnvironments handles requests to /api/v1/environments
func (r *Router) handleEnvironments(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.environmentHandler.GetEnvironments(w, req)
	case http.MethodPost:
		r.environmentHandler.CreateEnvironment(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEnvironmentByID handles requests to /api/v1/environments/{id}
func (r *Router) handleEnvironmentByID(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.environmentHandler.GetEnvironment(w, req)
	case http.MethodPut:
		r.environmentHandler.UpdateEnvironment(w, req)
	case http.MethodDelete:
		r.environmentHandler.DeleteEnvironment(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEnvironmentTenantMapping handles the mapping/unmapping of environments to tenants
func (r *Router) handleEnvironmentTenantMapping(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		r.environmentHandler.MapEnvironmentToTenant(w, req)
	case http.MethodDelete:
		r.environmentHandler.UnmapEnvironmentFromTenant(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListeners handles requests to /api/v1/listeners
func (r *Router) handleListeners(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listenerHandler.GetListeners(w, req)
	case http.MethodPost:
		r.listenerHandler.CreateListener(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListenerByID handles requests to /api/v1/listeners/{id}
func (r *Router) handleListenerByID(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listenerHandler.GetListener(w, req)
	case http.MethodPut:
		r.listenerHandler.UpdateListener(w, req)
	case http.MethodDelete:
		r.listenerHandler.DeleteListener(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListenerEnvironmentMapping handles the mapping/unmapping of listeners to environments
func (r *Router) handleListenerEnvironmentMapping(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		r.listenerHandler.MapListenerToEnvironment(w, req)
	case http.MethodDelete:
		r.listenerHandler.UnmapListenerFromEnvironment(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// corsMiddleware adds CORS headers
func (r *Router) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}
