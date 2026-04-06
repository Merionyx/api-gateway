package git

import (
	"strings"
	"testing"
)

func TestRepositoryManager_GetRepository_NotFound(t *testing.T) {
	rm := NewRepositoryManager()
	_, err := rm.GetRepository("missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestRepositoryManager_GetRepositorySnapshots_NotFound(t *testing.T) {
	rm := NewRepositoryManager()
	_, err := rm.GetRepositorySnapshots("missing", "heads/main", ".")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestRepositoryManager_InitializeRepositories_InvalidGitURL(t *testing.T) {
	rm := NewRepositoryManager()
	err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "bad", Source: RepositorySourceGit, URL: "http://127.0.0.1:1/unreachable.git"},
	})
	if err == nil {
		t.Fatal("expected clone error")
	}
}
