package bundle

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/pagination"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// BundleReadUseCase serves HTTP registry reads for etcd bundle keys and contract snapshots.
type BundleReadUseCase struct {
	snapshots interfaces.SnapshotRepository
}

func NewBundleReadUseCase(snapshots interfaces.SnapshotRepository) *BundleReadUseCase {
	return &BundleReadUseCase{snapshots: snapshots}
}

func (u *BundleReadUseCase) ListBundleKeys(ctx context.Context, limit *int, cursor *string) ([]string, *string, bool, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseBundlePkg, "ListBundleKeys"))
	defer span.End()
	keys, err := u.snapshots.ListBundleKeys(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, nil, false, err
	}
	lim := pagination.ResolveLimit(limit)
	return pagination.PageStringSlice(keys, lim, cursor)
}

func (u *BundleReadUseCase) ListContractNames(ctx context.Context, bundleKey string, limit *int, cursor *string) ([]string, *string, bool, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseBundlePkg, "ListContractNames"))
	defer span.End()
	snaps, err := u.snapshots.GetSnapshots(ctx, bundleKey)
	if err != nil {
		telemetry.MarkError(span, err)
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
	lim := pagination.ResolveLimit(limit)
	return pagination.PageStringSlice(names, lim, cursor)
}

// GetContractDocument returns the stored snapshot as a generic JSON object (canonical encoding).
func (u *BundleReadUseCase) GetContractDocument(ctx context.Context, bundleKey, contractName string) (map[string]interface{}, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseBundlePkg, "GetContractDocument"))
	defer span.End()
	snaps, err := u.snapshots.GetSnapshots(ctx, bundleKey)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, err
	}
	for _, s := range snaps {
		if s.Name != contractName {
			continue
		}
		raw, err := json.Marshal(s)
		if err != nil {
			telemetry.MarkError(span, err)
			return nil, err
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(raw, &doc); err != nil {
			telemetry.MarkError(span, err)
			return nil, err
		}
		return doc, nil
	}
	return nil, apierrors.ErrNotFound
}
