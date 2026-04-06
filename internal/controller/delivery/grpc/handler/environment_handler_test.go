package handler

import (
	"context"
	"testing"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/controller/index/bundleenv"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	environmentsv1 "merionyx/api-gateway/pkg/api/environments/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type noopEnvUC struct{}

func (noopEnvUC) SetDependencies(interfaces.EnvironmentRepository, interfaces.SchemasUseCase, *xdscache.SnapshotManager, interfaces.XDSBuilder) {
}

func (noopEnvUC) CreateEnvironment(context.Context, *models.CreateEnvironmentRequest) (*models.Environment, error) {
	return &models.Environment{Name: "x"}, nil
}
func (noopEnvUC) GetEnvironment(context.Context, string) (*models.Environment, error) { return nil, nil }
func (noopEnvUC) ListEnvironments(context.Context) (map[string]*models.Environment, error) {
	return nil, nil
}
func (noopEnvUC) UpdateEnvironment(context.Context, *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	return nil, nil
}
func (noopEnvUC) DeleteEnvironment(context.Context, string) error { return nil }

var _ interfaces.EnvironmentsUseCase = noopEnvUC{}

type followerGate struct{}

func (followerGate) IsLeader() bool                { return false }
func (followerGate) LeaderChanged() <-chan struct{} { return nil }

func TestEnvironmentsHandler_CreateEnvironment_NotLeader(t *testing.T) {
	h := NewEnvironmentsHandler(noopEnvUC{}, followerGate{}, bundleenv.NewIndex(nil, nil), false)
	_, err := h.CreateEnvironment(context.Background(), &environmentsv1.CreateEnvironmentRequest{Name: "n1"})
	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("code: %v", st.Code())
	}
}

func TestEnvironmentsHandler_CreateEnvironment_InvalidName(t *testing.T) {
	h := NewEnvironmentsHandler(noopEnvUC{}, nil, nil, false)
	_, err := h.CreateEnvironment(context.Background(), &environmentsv1.CreateEnvironmentRequest{Name: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code: %v", st.Code())
	}
}
