package sync

import (
	"strings"
	"testing"

	"merionyx/api-gateway/internal/auth-sidecar/config"
	"merionyx/api-gateway/internal/auth-sidecar/storage"
)

func TestNewSyncClient_SidecarID(t *testing.T) {
	cfg := &config.Config{
		Controller: config.ControllerConfig{Address: "127.0.0.1:1", Environment: "dev"},
	}
	st := storage.NewAccessStorage()
	c := NewSyncClient(cfg, st)
	if !strings.HasPrefix(c.sidecarID, "sidecar-") {
		t.Fatalf("sidecarID %q", c.sidecarID)
	}
}
