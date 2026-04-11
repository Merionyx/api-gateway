package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_emptyDir(t *testing.T) {
	t.Parallel()
	res := Run(context.Background(), t.TempDir(), Options{})
	if len(res) != 1 || !res[0].Skipped {
		t.Fatalf("got %#v", res)
	}
}

func TestRun_missingPath(t *testing.T) {
	t.Parallel()
	res := Run(context.Background(), filepath.Join(t.TempDir(), "nope"), Options{})
	if len(res) != 1 || !res[0].Skipped {
		t.Fatalf("got %#v", res)
	}
}

func TestRun_validContractFile(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "c.yaml")
	doc := `x-api-gateway:
  version: v1
  prefix: /api/x/
  allowUndefinedMethods: false
  contract:
    name: c
  service:
    name: s
  access:
    secure: true
`
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	res := Run(context.Background(), p, Options{})
	if len(res) != 1 || len(res[0].Issues) != 0 || res[0].Skipped {
		t.Fatalf("got %#v", res)
	}
}

func TestRun_openAPIWithContentCheck(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "api.yaml")
	doc := `openapi: "3.0.0"
info:
  title: T
  version: "1.0"
paths: {}
x-api-gateway:
  version: v1
  prefix: /api/x/
  allowUndefinedMethods: false
  contract:
    name: c
  service:
    name: s
  access:
    secure: true
`
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	res := Run(context.Background(), p, Options{CheckContent: true})
	if len(res) != 1 || len(res[0].Issues) != 0 || res[0].Skipped {
		t.Fatalf("got %#v", res)
	}
}
