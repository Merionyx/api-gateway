package usecase

import (
	"testing"

	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

func TestSyncUseCase_Sync_UnknownRepo(t *testing.T) {
	u := NewSyncUseCase(sharedgit.NewRepositoryManager(), false)
	_, err := u.Sync("missing", "ref", "path")
	if err == nil {
		t.Fatal("expected error")
	}
}
