package cache

import (
	"context"
	"fmt"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	res "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

type SnapshotManager struct {
	cache cache.SnapshotCache
}

func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		cache: cache.NewSnapshotCache(false, cache.IDHash{}, nil),
	}
}

func (sm *SnapshotManager) UpdateSnapshot(nodeID string, snapshot *cache.Snapshot) error {
	prev, err := sm.cache.GetSnapshot(nodeID)
	if err == nil {
		if ps, ok := prev.(*cache.Snapshot); ok {
			pv := ps.GetVersion(res.ClusterType)
			nv := snapshot.GetVersion(res.ClusterType)
			if pv != "" && nv != "" && pv == nv {
				return nil
			}
		}
	}
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
