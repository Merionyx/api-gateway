package registry

import (
	"context"
	"testing"
)

func TestParseBundleKeyFromSnapshotKey(t *testing.T) {
	t.Parallel()
	if _, ok := parseBundleKeyFromSnapshotKey("/other/prefix/foo/contracts/x"); ok {
		t.Fatal("expected false")
	}
	if _, ok := parseBundleKeyFromSnapshotKey("/api-gateway/api-server/snapshots/bk"); ok {
		t.Fatal("expected false without /contracts/")
	}
	k, ok := parseBundleKeyFromSnapshotKey("/api-gateway/api-server/snapshots/org%2Frepo%2Fref/contracts/api.yaml")
	if !ok || k != "org%2Frepo%2Fref" {
		t.Fatalf("got %q ok=%v", k, ok)
	}
}

func TestParseControllerIDKey(t *testing.T) {
	t.Parallel()
	if _, ok := parseControllerIDKey("/api-gateway/api-server/controllers"); ok {
		t.Fatal("expected false for prefix only")
	}
	if _, ok := parseControllerIDKey("/api-gateway/api-server/controllers/c1/sub"); ok {
		t.Fatal("expected false for nested path")
	}
	id, ok := parseControllerIDKey("/api-gateway/api-server/controllers/c1")
	if !ok || id != "c1" {
		t.Fatalf("got %q ok=%v", id, ok)
	}
}

func TestStartEtcdWatch_nilClient(t *testing.T) {
	t.Parallel()
	uc := NewControllerRegistryUseCase(nil, nil, nil)
	uc.StartEtcdWatch(context.Background())
}
