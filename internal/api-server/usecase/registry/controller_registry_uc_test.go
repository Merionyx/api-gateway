package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

func TestControllerRegistryUseCase_RegisterController(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	uc := NewControllerRegistryUseCase(&stubControllerRepo{}, nil, nil)
	if err := uc.RegisterController(ctx, models.ControllerInfo{ControllerID: "c"}); err != nil {
		t.Fatal(err)
	}
	uc2 := NewControllerRegistryUseCase(&stubControllerRepo{registerErr: errors.New("fail")}, nil, nil)
	if err := uc2.RegisterController(ctx, models.ControllerInfo{ControllerID: "c"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestControllerRegistryUseCase_NotifySnapshotUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	uc := NewControllerRegistryUseCase(&stubControllerRepo{err: errors.New("list")}, nil, nil)
	if err := uc.NotifySnapshotUpdate(ctx, "bk", nil); err == nil {
		t.Fatal("expected error")
	}
	uc2 := NewControllerRegistryUseCase(&stubControllerRepo{}, nil, nil)
	if err := uc2.NotifySnapshotUpdate(ctx, "bk", nil); err != nil {
		t.Fatal(err)
	}
}

func TestControllerRegistryUseCase_Heartbeat_noResync(t *testing.T) {
	t.Parallel()
	uc := NewControllerRegistryUseCase(&stubControllerRepo{}, nil, nil)
	if err := uc.Heartbeat(context.Background(), "c1", nil, 0); err != nil {
		t.Fatal(err)
	}
}
