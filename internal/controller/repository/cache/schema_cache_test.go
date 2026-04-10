package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeSchemaRepo struct {
	listCalls int
	listOut   []models.ContractSnapshot
	listErr   error
	saveErr   error
}

func (f *fakeSchemaRepo) ListContractSnapshots(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error) {
	f.listCalls++
	return f.listOut, f.listErr
}

func (f *fakeSchemaRepo) SaveContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string, snapshot *models.ContractSnapshot) error {
	return f.saveErr
}

func (f *fakeSchemaRepo) GetContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string) (*models.ContractSnapshot, error) {
	return nil, errors.New("not used")
}

func (f *fakeSchemaRepo) GetEnvironmentSnapshots(ctx context.Context, envName string) ([]models.ContractSnapshot, error) {
	return nil, errors.New("not used")
}

func (f *fakeSchemaRepo) WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan {
	return nil
}

func TestSchemaCache_ListContractSnapshots_hitSecondCall(t *testing.T) {
	inner := &fakeSchemaRepo{
		listOut: []models.ContractSnapshot{{Name: "c1", Prefix: "/p"}},
	}
	c := NewSchemaCache(inner, false)
	ctx := context.Background()

	_, err := c.ListContractSnapshots(ctx, "r", "main", "")
	if err != nil || inner.listCalls != 1 {
		t.Fatalf("first call: err=%v calls=%d", err, inner.listCalls)
	}
	_, err = c.ListContractSnapshots(ctx, "r", "main", "")
	if err != nil || inner.listCalls != 1 {
		t.Fatalf("second call should hit cache: err=%v calls=%d", err, inner.listCalls)
	}
}

func TestSchemaCache_InvalidateBundle_refetch(t *testing.T) {
	inner := &fakeSchemaRepo{
		listOut: []models.ContractSnapshot{{Name: "c1"}},
	}
	c := NewSchemaCache(inner, false)
	ctx := context.Background()
	_, _ = c.ListContractSnapshots(ctx, "r", "main", "pkg")
	c.InvalidateBundle("r", "main", "pkg")
	_, _ = c.ListContractSnapshots(ctx, "r", "main", "pkg")
	if inner.listCalls != 2 {
		t.Fatalf("expected 2 inner calls after invalidate, got %d", inner.listCalls)
	}
}

func TestSchemaCache_SaveContractSnapshot_invalidates(t *testing.T) {
	inner := &fakeSchemaRepo{listOut: []models.ContractSnapshot{{Name: "x"}}}
	c := NewSchemaCache(inner, false)
	ctx := context.Background()
	_, _ = c.ListContractSnapshots(ctx, "r", "main", "")
	_ = c.SaveContractSnapshot(ctx, "r", "main", "", "x", &models.ContractSnapshot{Name: "x"})
	_, _ = c.ListContractSnapshots(ctx, "r", "main", "")
	if inner.listCalls != 2 {
		t.Fatalf("Save should invalidate: want 2 list calls, got %d", inner.listCalls)
	}
}

func TestSchemaCache_cloneContractSnapshots_isolation(t *testing.T) {
	inner := &fakeSchemaRepo{
		listOut: []models.ContractSnapshot{
			{
				Name: "n",
				Access: models.Access{
					Apps: []models.App{{AppID: "a", Environments: []string{"e"}}},
				},
			},
		},
	}
	c := NewSchemaCache(inner, false)
	ctx := context.Background()
	a, _ := c.ListContractSnapshots(ctx, "r", "main", "")
	b, _ := c.ListContractSnapshots(ctx, "r", "main", "")
	a[0].Access.Apps[0].AppID = "mutated"
	if b[0].Access.Apps[0].AppID != "a" {
		t.Fatal("cached copy should be independent")
	}
}
