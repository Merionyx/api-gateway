package handler

import (
	"context"
	"errors"
	"testing"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type noopRegistryUC struct{}

func (noopRegistryUC) RegisterController(context.Context, models.ControllerInfo) error { return nil }
func (noopRegistryUC) StreamSnapshots(context.Context, string, interfaces.SnapshotStream) error {
	return nil
}
func (noopRegistryUC) Heartbeat(context.Context, string, []models.EnvironmentInfo) error { return nil }
func (noopRegistryUC) StartEtcdWatch(context.Context)                                    {}

func TestControllerRegistryHandler_RegisterController_Success(t *testing.T) {
	h := NewControllerRegistryHandler(noopRegistryUC{}, false)
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

type errRegistryUC struct{ err error }

func (e errRegistryUC) RegisterController(context.Context, models.ControllerInfo) error { return e.err }
func (errRegistryUC) StreamSnapshots(context.Context, string, interfaces.SnapshotStream) error {
	return nil
}
func (errRegistryUC) Heartbeat(context.Context, string, []models.EnvironmentInfo) error { return nil }
func (errRegistryUC) StartEtcdWatch(context.Context)                                    {}

func TestControllerRegistryHandler_RegisterController_StatusError(t *testing.T) {
	h := NewControllerRegistryHandler(errRegistryUC{err: apierrors.JoinStore("register controller", errors.New("etcd down"))}, false)
	_, err := h.RegisterController(context.Background(), &pb.RegisterControllerRequest{
		ControllerId: "c1",
		Tenant:       "t1",
	})
	if err == nil {
		t.Fatal("expected gRPC status error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected status.FromError, got %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Fatalf("code: %v", st.Code())
	}
}

type heartbeatErrUC struct{ err error }

func (heartbeatErrUC) RegisterController(context.Context, models.ControllerInfo) error { return nil }
func (heartbeatErrUC) StreamSnapshots(context.Context, string, interfaces.SnapshotStream) error {
	return nil
}
func (h heartbeatErrUC) Heartbeat(context.Context, string, []models.EnvironmentInfo) error {
	return h.err
}
func (heartbeatErrUC) StartEtcdWatch(context.Context) {}

func TestControllerRegistryHandler_Heartbeat_Success(t *testing.T) {
	h := NewControllerRegistryHandler(noopRegistryUC{}, false)
	resp, err := h.Heartbeat(context.Background(), &pb.HeartbeatRequest{ControllerId: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("resp: %+v", resp)
	}
}

func TestControllerRegistryHandler_Heartbeat_StatusError(t *testing.T) {
	h := NewControllerRegistryHandler(heartbeatErrUC{err: apierrors.ErrInvalidInput}, false)
	_, err := h.Heartbeat(context.Background(), &pb.HeartbeatRequest{ControllerId: "c1"})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v ok=%v", err, ok)
	}
}
