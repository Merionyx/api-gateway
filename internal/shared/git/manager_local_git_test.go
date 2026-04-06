package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepositoryManager_LocalGitSnapshotsAndCache(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repoDir := t.TempDir()
	openapiDir := filepath.Join(repoDir, "openapi")
	if err := os.MkdirAll(openapiDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yaml := `openapi: 3.0.0
info:
  title: t
  version: "1"
paths: {}
x-api-gateway:
  prefix: /api/v1
  allow_undefined_methods: false
  contract:
    name: contract-a
  service:
    name: upstream-be
  access:
    secure: true
    apps:
      - app_id: app1
        environments: [dev]
`
	if err := os.WriteFile(filepath.Join(openapiDir, "api.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init", "--no-gpg-sign")

	abs, err := filepath.Abs(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	fileURL := "file://" + filepath.ToSlash(abs)

	rm := NewRepositoryManager()
	if err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "lg", Source: RepositorySourceLocalGit, Path: fileURL},
	}); err != nil {
		t.Fatalf("InitializeRepositories: %v", err)
	}

	ref := "heads/main"
	sub := "openapi"
	snaps1, err := rm.GetRepositorySnapshots("lg", ref, sub)
	if err != nil {
		t.Fatalf("GetRepositorySnapshots 1: %v", err)
	}
	if len(snaps1) != 1 || snaps1[0].Name != "contract-a" {
		t.Fatalf("unexpected %+v", snaps1)
	}

	snaps2, err := rm.GetRepositorySnapshots("lg", ref, sub)
	if err != nil {
		t.Fatalf("GetRepositorySnapshots 2: %v", err)
	}
	if len(snaps2) != 1 || snaps2[0].Name != "contract-a" {
		t.Fatalf("unexpected second %+v", snaps2)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
