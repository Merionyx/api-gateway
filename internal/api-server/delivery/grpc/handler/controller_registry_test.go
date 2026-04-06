package handler

import (
	"context"
	"testing"

	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/domain/models"
	pb "merionyx/api-gateway/pkg/api/controller_registry/v1"
)

type noopRegistryUC struct{}

func (noopRegistryUC) RegisterController(context.Context, models.ControllerInfo) error { return nil }
func (noopRegistryUC) StreamSnapshots(context.Context, string, interfaces.SnapshotStream) error {
	return nil
}
func (noopRegistryUC) Heartbeat(context.Context, string, []models.EnvironmentInfo) error { return nil }
func (noopRegistryUC) StartEtcdWatch(context.Context) {}

func TestControllerRegistryHandler_RegisterController_Success(t *testing.T) {
	h := NewControllerRegistryHandler(noopRegistryUC{})
	resp, err := h.RegisterController(context.Background(), &pb.RegisterControllerRequest{
		ControllerId: "c1",
		Tenant:       "t1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("resp: %+v", resp)
	}
}
