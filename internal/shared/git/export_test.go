package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const exportContractYAML = `openapi: 3.0.0
info:
  title: t
  version: "1"
paths: {}
x-api-gateway:
  prefix: /p
  contract:
    name: dup-contract
  service:
    name: svc
  access:
    secure: false
`

func TestExportContractFiles_DuplicateContractName(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "a.yaml"), []byte(exportContractYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "b.yaml"), []byte(exportContractYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init", "--no-gpg-sign")

	abs, err := filepath.Abs(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	fileURL := "file://" + filepath.ToSlash(abs)

	rm := NewRepositoryManager()
	if err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "ex", Source: RepositorySourceLocalGit, Path: fileURL},
	}); err != nil {
		t.Fatal(err)
	}

	_, err = rm.ExportContractFiles(context.Background(), "ex", "heads/main", "", "")
	if err == nil {
		t.Fatal("expected duplicate contract name error")
	}
	if !strings.Contains(err.Error(), "duplicate contract name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExportContractFiles_SingleFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	repoDir := t.TempDir()
	yaml := `openapi: 3.0.0
info:
  title: t
  version: "1"
paths: {}
x-api-gateway:
  prefix: /api/v1
  contract:
    name: only-one
  service:
    name: upstream-be
  access:
    secure: true
`
	if err := os.WriteFile(filepath.Join(repoDir, "spec.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init", "--no-gpg-sign")

	abs, err := filepath.Abs(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	fileURL := "file://" + filepath.ToSlash(abs)

	rm := NewRepositoryManager()
	if err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "ex", Source: RepositorySourceLocalGit, Path: fileURL},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := rm.ExportContractFiles(context.Background(), "ex", "heads/main", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ContractName != "only-one" || string(out[0].Content) != yaml {
		t.Fatalf("unexpected: %+v", out)
	}

	_, err = rm.ExportContractFiles(context.Background(), "ex", "heads/main", "", "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found: %v", err)
	}

	one, err := rm.ExportContractFiles(context.Background(), "ex", "heads/main", "", "only-one")
	if err != nil || len(one) != 1 {
		t.Fatalf("filter: %v %+v", err, one)
	}
}
