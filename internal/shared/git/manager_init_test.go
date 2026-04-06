package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryManager_LocalDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte("x: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rm := NewRepositoryManager()
	err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "local", Source: RepositorySourceLocalDir, Path: dir},
	})
	if err != nil {
		t.Fatalf("InitializeRepositories: %v", err)
	}
}

func TestRepositoryManager_UnsupportedSource(t *testing.T) {
	rm := NewRepositoryManager()
	err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "bad", Source: "unknown", Path: "/tmp"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
