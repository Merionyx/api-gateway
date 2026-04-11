package usecase

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
)

// BundleReadUseCase serves HTTP registry reads for etcd bundle keys and contract snapshots.
type BundleReadUseCase struct {
	snapshots interfaces.SnapshotRepository
}

func NewBundleReadUseCase(snapshots interfaces.SnapshotRepository) *BundleReadUseCase {
	return &BundleReadUseCase{snapshots: snapshots}
}

func (u *BundleReadUseCase) ListBundleKeys(ctx context.Context, limit *int, cursor *string) ([]string, *string, bool, error) {
	keys, err := u.snapshots.ListBundleKeys(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	lim := ResolveLimit(limit)
	return PageStringSlice(keys, lim, cursor)
}

func (u *BundleReadUseCase) ListContractNames(ctx context.Context, bundleKey string, limit *int, cursor *string) ([]string, *string, bool, error) {
	snaps, err := u.snapshots.GetSnapshots(ctx, bundleKey)
	if err != nil {
		return nil, nil, false, err
	}
	names := make([]string, 0, len(snaps))
	seen := make(map[string]struct{}, len(snaps))
	for _, s := range snaps {
		if _, ok := seen[s.Name]; ok {
			continue
		}
		seen[s.Name] = struct{}{}
		names = append(names, s.Name)
	}
	sort.Strings(names)
	lim := ResolveLimit(limit)
	return PageStringSlice(names, lim, cursor)
}

// GetContractDocument returns the stored snapshot as a generic JSON object (canonical encoding).
func (u *BundleReadUseCase) GetContractDocument(ctx context.Context, bundleKey, contractName string) (map[string]interface{}, error) {
	snaps, err := u.snapshots.GetSnapshots(ctx, bundleKey)
	if err != nil {
		return nil, err
	}
	for _, s := range snaps {
		if s.Name != contractName {
			continue
		}
		raw, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		return doc, nil
	}
	return nil, apierrors.ErrNotFound
}
