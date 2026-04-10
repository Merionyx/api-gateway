package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

func Run() error {
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "unknown-service"
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "unknown"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	hostname, _ := os.Hostname()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if timeoutStr := r.URL.Query().Get("timeout"); timeoutStr != "" {
			if timeoutMs, err := strconv.Atoi(timeoutStr); err == nil && timeoutMs > 0 {
				duration := time.Duration(timeoutMs) * time.Millisecond
				slog.Info("mock service sleeping before respond", "environment", environment, "service", serviceName, "duration", duration)
				time.Sleep(duration)
			}
		}
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}
		query := make(map[string]string)
		for name, values := range r.URL.Query() {
			if len(values) > 0 {
				query[name] = values[0]
			}
		}
		response := Response{
			Service:     serviceName,
			Environment: environment,
			Timestamp:   time.Now().Format(time.RFC3339),
			Path:        r.URL.Path,
			Method:      r.Method,
			Headers:     headers,
			Query:       query,
			Host:        hostname,
		}
		slog.Info("mock service request",
			"environment", environment,
			"service", serviceName,
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Service-Name", serviceName)
		w.Header().Set("X-Environment", environment)
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("mock service encode response", "error", err)
		}
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, "OK"); err != nil {
			slog.Error("mock service health write", "error", err)
		}
	})

	addr := ":" + port
	slog.Info("starting mock service", "service", serviceName, "environment", environment, "addr", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}
