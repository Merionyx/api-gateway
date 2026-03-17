package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Response struct {
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Timestamp   string            `json:"timestamp"`
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Query       map[string]string `json:"query"`
	Host        string            `json:"host"`
}

func main() {
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
		// Check if timeout parameter is present
		if timeoutStr := r.URL.Query().Get("timeout"); timeoutStr != "" {
			if timeoutMs, err := strconv.Atoi(timeoutStr); err == nil && timeoutMs > 0 {
				duration := time.Duration(timeoutMs) * time.Millisecond
				log.Printf("[%s][%s] Sleeping for %v before responding", environment, serviceName, duration)
				time.Sleep(duration)
			}
		}
		// Collect headers
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}
		// Collect query parameters
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
		log.Printf("[%s][%s] %s %s from %s",
			environment,
			serviceName,
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
		)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Service-Name", serviceName)
		w.Header().Set("X-Environment", environment)
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(response)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	addr := ":" + port
	log.Printf("Starting %s service in %s environment on %s", serviceName, environment, addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
