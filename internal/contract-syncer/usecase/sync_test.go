package usecase

import (
	"context"
	"testing"

	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

func TestSyncUseCase_Sync_UnknownRepo(t *testing.T) {
	u := NewSyncUseCase(sharedgit.NewRepositoryManager(), false)
	_, err := u.Sync(context.Background(), "missing", "ref", "path")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSyncUseCase_ExportContracts_Unknown(t *testing.T) {
	t.Parallel()
	u := NewSyncUseCase(sharedgit.NewRepositoryManager(), true)
	_, err := u.ExportContracts(context.Background(), "missing", "ref", "path", "c")
	if err == nil {
		t.Fatal("expected error")
	}
}
