package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

type SnapshotManager struct {
	cache cache.SnapshotCache
	mu    sync.RWMutex
}

func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		cache: cache.NewSnapshotCache(false, cache.IDHash{}, nil),
	}
}

func (sm *SnapshotManager) UpdateSnapshot(nodeID string, snapshot *cache.Snapshot) error {
	return sm.cache.SetSnapshot(context.Background(), nodeID, snapshot)
}

func (sm *SnapshotManager) GetCache() cache.SnapshotCache {
	return sm.cache
}

func (sm *SnapshotManager) GetSnapshot(nodeID string) (*cache.Snapshot, error) {
	snapshot, err := sm.cache.GetSnapshot(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}
	return snapshot.(*cache.Snapshot), nil
}

func (sm *SnapshotManager) DeleteSnapshot(nodeID string) error {
	sm.cache.ClearSnapshot(nodeID)
	return nil
}
