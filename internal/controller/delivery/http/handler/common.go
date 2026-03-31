package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorResponse standard error response structure
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SuccessResponse standard success response structure
type SuccessResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// writeJSONResponse writes a JSON response
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeErrorResponse writes an error response
func writeSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	writeJSONResponse(w, statusCode, SuccessResponse{
		Data: data,
	})
}

// writeErrorResponse writes an error response
func writeSuccessMessage(w http.ResponseWriter, statusCode int, message string) {
	writeJSONResponse(w, statusCode, SuccessResponse{
		Message: message,
	})
}

// writeErrorResponse writes an error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, error, message string) {
	writeJSONResponse(w, statusCode, ErrorResponse{
		Error:   error,
		Message: message,
	})
}

// getPathParam extracts a parameter from the URL path
func getPathParam(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	param := strings.TrimPrefix(path, prefix)
	// Remove everything after the next slash
	if idx := strings.Index(param, "/"); idx != -1 {
		param = param[:idx]
	}

	return param
}

// getPathParamAfter extracts a parameter after the specified prefix
func getPathParamAfter(path, prefix string) string {
	idx := strings.Index(path, prefix)
	if idx == -1 {
		return ""
	}

	param := path[idx+len(prefix):]
	// Remove everything after the next slash
	if idx := strings.Index(param, "/"); idx != -1 {
		param = param[:idx]
	}

	return param
}
