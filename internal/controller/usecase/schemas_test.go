package usecase

import (
	"context"
	"errors"
	"testing"

	"merionyx/api-gateway/internal/controller/domain/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestSchemasUseCase_SyncContractBundle_ErrManagedByAPI(t *testing.T) {
	uc := NewSchemasUseCase().(*schemasUseCase)
	_, err := uc.SyncContractBundle(context.Background(), &models.SyncContractBundleRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrContractsManagedByAPIServer) {
		t.Fatalf("want ErrContractsManagedByAPIServer, got %v", err)
	}
}

func TestSchemasUseCase_SyncAllContracts_ErrManagedByAPI(t *testing.T) {
	uc := NewSchemasUseCase().(*schemasUseCase)
	_, err := uc.SyncAllContracts(context.Background(), &models.SyncAllContractsRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrContractsManagedByAPIServer) {
		t.Fatalf("want ErrContractsManagedByAPIServer, got %v", err)
	}
}

type fakeSchemaRepo struct {
	list  []models.ContractSnapshot
	get   *models.ContractSnapshot
	watch clientv3.WatchChan
}

func (f *fakeSchemaRepo) SaveContractSnapshot(context.Context, string, string, string, string, *models.ContractSnapshot) error {
	return nil
}
func (f *fakeSchemaRepo) GetContractSnapshot(context.Context, string, string, string, string) (*models.ContractSnapshot, error) {
	return f.get, nil
}
func (f *fakeSchemaRepo) GetEnvironmentSnapshots(context.Context, string) ([]models.ContractSnapshot, error) {
	return nil, nil
}
func (f *fakeSchemaRepo) ListContractSnapshots(context.Context, string, string, string) ([]models.ContractSnapshot, error) {
	return f.list, nil
}
func (f *fakeSchemaRepo) WatchContractBundlesSnapshots(context.Context) clientv3.WatchChan {
	return f.watch
}

func TestSchemasUseCase_ListContractSnapshots_Delegates(t *testing.T) {
	want := []models.ContractSnapshot{{Name: "c1"}}
	uc := NewSchemasUseCase().(*schemasUseCase)
	uc.SetDependencies(&fakeSchemaRepo{list: want}, nil)
	got, err := uc.ListContractSnapshots(context.Background(), "r", "ref", "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "c1" {
		t.Fatalf("got %+v", got)
	}
}
