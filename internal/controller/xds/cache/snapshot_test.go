package cache

import (
	"testing"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

func TestSnapshotManager_GetSnapshot_Missing(t *testing.T) {
	sm := NewSnapshotManager(false)
	_, err := sm.GetSnapshot("unknown-node")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSnapshotManager_UpdateSnapshot_Minimal(t *testing.T) {
	sm := NewSnapshotManager(false)
	snap, err := envoycache.NewSnapshot("v1", map[resource.Type][]types.Resource{
		resource.ClusterType: {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.UpdateSnapshot("node-a", snap); err != nil {
		t.Fatal(err)
	}
	got, err := sm.GetSnapshot("node-a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("nil snapshot")
	}
}
