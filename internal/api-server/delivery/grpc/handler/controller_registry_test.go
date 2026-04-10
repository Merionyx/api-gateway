package handler

import (
	"context"
	"testing"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

type noopRegistryUC struct{}

func (noopRegistryUC) RegisterController(context.Context, models.ControllerInfo) error { return nil }
func (noopRegistryUC) StreamSnapshots(context.Context, string, interfaces.SnapshotStream) error {
	return nil
}
func (noopRegistryUC) Heartbeat(context.Context, string, []models.EnvironmentInfo) error { return nil }
func (noopRegistryUC) StartEtcdWatch(context.Context)                                    {}

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
