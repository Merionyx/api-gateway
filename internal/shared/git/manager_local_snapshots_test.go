package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryManager_GetRepositorySnapshots_LocalDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "openapi")
	if err := os.MkdirAll(sub, 0o700); err != nil {
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
	if err := os.WriteFile(filepath.Join(sub, "api.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	rm := NewRepositoryManager()
	if err := rm.InitializeRepositories([]RepositoryConfig{
		{Name: "schemas", Source: RepositorySourceLocalDir, Path: dir},
	}); err != nil {
		t.Fatal(err)
	}
	snaps, err := rm.GetRepositorySnapshots(context.Background(), "schemas", "", "openapi")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Name != "contract-a" || snaps[0].Prefix != "/api/v1" {
		t.Fatalf("%+v", snaps)
	}
}
