package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	sharedgit "merionyx/api-gateway/internal/shared/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	snapshotPrefix = "/api-gateway/api-server/snapshots/"
)

type SnapshotRepository struct {
	client *clientv3.Client
}

func NewSnapshotRepository(client *clientv3.Client) *SnapshotRepository {
	return &SnapshotRepository{
		client: client,
	}
}

func (r *SnapshotRepository) SaveSnapshots(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	slog.Info("Saving snapshots to etcd", "bundle_key", bundleKey, "count", len(snapshots))

	for _, snapshot := range snapshots {
		key := fmt.Sprintf("%s%s/contracts/%s", snapshotPrefix, bundleKey, snapshot.Name)
		
		data, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("failed to marshal snapshot: %w", err)
		}

		_, err = r.client.Put(ctx, key, string(data))
		if err != nil {
			return fmt.Errorf("failed to save snapshot to etcd: %w", err)
		}

		slog.Debug("Saved snapshot", "key", key, "name", snapshot.Name)
	}

	return nil
}

func (r *SnapshotRepository) GetSnapshots(ctx context.Context, bundleKey string) ([]sharedgit.ContractSnapshot, error) {
	prefix := fmt.Sprintf("%s%s/contracts/", snapshotPrefix, bundleKey)
	
	slog.Debug("Getting snapshots from etcd", "prefix", prefix)
	
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots from etcd: %w", err)
	}

	slog.Debug("Got response from etcd", "keys_count", len(resp.Kvs))

	var snapshots []sharedgit.ContractSnapshot
	for _, kv := range resp.Kvs {
		slog.Debug("Processing snapshot", "key", string(kv.Key))
		
		var snapshot sharedgit.ContractSnapshot
		if err := json.Unmarshal(kv.Value, &snapshot); err != nil {
			slog.Error("Failed to unmarshal snapshot", "key", string(kv.Key), "error", err)
			continue
		}
		snapshots = append(snapshots, snapshot)
	}

	slog.Debug("Returning snapshots", "count", len(snapshots))

	return snapshots, nil
}

func (r *SnapshotRepository) ListBundleKeys(ctx context.Context) ([]string, error) {
	resp, err := r.client.Get(ctx, snapshotPrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, fmt.Errorf("failed to list bundle keys from etcd: %w", err)
	}

	bundleKeysMap := make(map[string]bool)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		key = strings.TrimPrefix(key, snapshotPrefix)
		parts := strings.Split(key, "/")
		if len(parts) > 0 {
			bundleKeysMap[parts[0]] = true
		}
	}

	var bundleKeys []string
	for bundleKey := range bundleKeysMap {
		bundleKeys = append(bundleKeys, bundleKey)
	}

	return bundleKeys, nil
}
