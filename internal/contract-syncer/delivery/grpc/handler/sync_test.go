package handler

import (
	"context"
	"errors"
	"testing"

	"merionyx/api-gateway/internal/contract-syncer/domain/interfaces"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"
)

type fakeSyncUC struct {
	err    error
	snaps  []sharedgit.ContractSnapshot
	export []sharedgit.ExportedContractFile
	expErr error
}

func (f *fakeSyncUC) Sync(string, string, string) ([]sharedgit.ContractSnapshot, error) {
	return f.snaps, f.err
}

func (f *fakeSyncUC) ExportContracts(string, string, string, string) ([]sharedgit.ExportedContractFile, error) {
	return f.export, f.expErr
}

var _ interfaces.SyncUseCase = (*fakeSyncUC)(nil)

func TestSyncHandler_Sync_ErrorInResponse(t *testing.T) {
	h := NewSyncHandler(&fakeSyncUC{err: errors.New("boom")}, false)
	resp, err := h.Sync(context.Background(), &pb.SyncRequest{Repository: "r", Ref: "main", Path: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected error string")
	}
}

func TestSyncHandler_Sync_OK(t *testing.T) {
	h := NewSyncHandler(&fakeSyncUC{snaps: []sharedgit.ContractSnapshot{
		{Name: "c1", Prefix: "/api", Upstream: sharedgit.ContractUpstream{Name: "u"}},
	}}, false)
	resp, err := h.Sync(context.Background(), &pb.SyncRequest{Repository: "r", Ref: "main", Path: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" || len(resp.Snapshots) != 1 {
		t.Fatalf("resp %+v", resp)
	}
}
