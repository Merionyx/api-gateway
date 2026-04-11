package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

type stubControllerRepo struct {
	list     []models.ControllerInfo
	err      error
	get      *models.ControllerInfo
	getErr   error
	hbeat    time.Time
	hbeatErr error

	registerErr error

	heartbeatUpdated   bool
	updateHeartbeatErr error
}

func (s *stubControllerRepo) RegisterController(context.Context, models.ControllerInfo) error {
	return s.registerErr
}

func (s *stubControllerRepo) GetController(context.Context, string) (*models.ControllerInfo, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.get, nil
}

func (s *stubControllerRepo) GetHeartbeat(context.Context, string) (time.Time, error) {
	if s.hbeatErr != nil {
		return time.Time{}, s.hbeatErr
	}
	return s.hbeat, nil
}

func (s *stubControllerRepo) ListControllers(context.Context) ([]models.ControllerInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

func (s *stubControllerRepo) UpdateControllerHeartbeat(context.Context, string, []models.EnvironmentInfo) (bool, error) {
	if s.updateHeartbeatErr != nil {
		return false, s.updateHeartbeatErr
	}
	return s.heartbeatUpdated, nil
}

func TestControllerReadUseCase_ListControllers_sortAndError(t *testing.T) {
	t.Parallel()
	u := NewControllerReadUseCase(&stubControllerRepo{
		list: []models.ControllerInfo{
			{ControllerID: "b"},
			{ControllerID: "a"},
		},
	})
	out, _, _, err := u.ListControllers(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ControllerID != "a" || out[1].ControllerID != "b" {
		t.Fatalf("got %#v", out)
	}

	u2 := NewControllerReadUseCase(&stubControllerRepo{err: errors.New("boom")})
	_, _, _, err = u2.ListControllers(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestControllerReadUseCase_GetController_GetHeartbeat(t *testing.T) {
	t.Parallel()
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	info := &models.ControllerInfo{ControllerID: "c1"}
	u := NewControllerReadUseCase(&stubControllerRepo{get: info, hbeat: ts})
	got, err := u.GetController(context.Background(), "c1")
	if err != nil || got.ControllerID != "c1" {
		t.Fatalf("get: %v %#v", err, got)
	}
	hb, err := u.GetHeartbeat(context.Background(), "c1")
	if err != nil || !hb.Equal(ts) {
		t.Fatalf("heartbeat: %v %v", err, hb)
	}
}

func TestControllerReadUseCase_ListControllersByTenant(t *testing.T) {
	t.Parallel()
	u := NewControllerReadUseCase(&stubControllerRepo{
		list: []models.ControllerInfo{
			{ControllerID: "x", Tenant: "t1"},
			{ControllerID: "y", Tenant: "t2"},
			{ControllerID: "z", Tenant: "t1"},
		},
	})
	out, _, _, err := u.ListControllersByTenant(context.Background(), "t1", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ControllerID != "x" || out[1].ControllerID != "z" {
		t.Fatalf("got %#v", out)
	}
}
