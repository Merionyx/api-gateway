package etcd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const spanAPIEtcdPkg = "internal/api-server/adapter/etcd"

const (
	snapshotPrefix    = "/api-gateway/api-server/snapshots/"
	maxSnapshotTxnOps = 128
)

type SnapshotRepository struct {
	client *clientv3.Client
}

func NewSnapshotRepository(client *clientv3.Client) *SnapshotRepository {
	return &SnapshotRepository{
		client: client,
	}
}

// snapshotPut is a planned Put for etcd (used by SaveSnapshots and tests).
type snapshotPut struct {
	key string
	val string
}

// buildSnapshotSavePlan compares desired snapshots with existing etcd values and returns puts/deletes.
func buildSnapshotSavePlan(bundleKey string, existingByName map[string][]byte, snapshots []sharedgit.ContractSnapshot) (puts []snapshotPut, dels []string, err error) {
	desired := make(map[string]struct{}, len(snapshots))
	for _, snapshot := range snapshots {
		desired[snapshot.Name] = struct{}{}
		key := fmt.Sprintf("%s%s/contracts/%s", snapshotPrefix, bundleKey, snapshot.Name)
		data, err := json.Marshal(snapshot)
		if err != nil {
			return nil, nil, apierrors.JoinStore("marshal snapshot", err)
		}
		if prev, ok := existingByName[snapshot.Name]; ok && bytes.Equal(prev, data) {
			continue
		}
		puts = append(puts, snapshotPut{key: key, val: string(data)})
	}

	for name := range existingByName {
		if _, ok := desired[name]; ok {
			continue
		}
		dels = append(dels, fmt.Sprintf("%s%s/contracts/%s", snapshotPrefix, bundleKey, name))
	}
	return puts, dels, nil
}

func (r *SnapshotRepository) SaveSnapshots(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) (written bool, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "SaveSnapshots"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	slog.Info("Saving snapshots to etcd", "bundle_key", bundleKey, "count", len(snapshots))

	contractsPrefix := fmt.Sprintf("%s%s/contracts/", snapshotPrefix, bundleKey)
	listResp, err := r.client.Get(ctx, contractsPrefix, clientv3.WithPrefix())
	if err != nil {
		return false, apierrors.JoinStore("list contract keys", err)
	}

	existingByName := make(map[string][]byte)
	for _, kv := range listResp.Kvs {
		keyStr := string(kv.Key)
		if !strings.HasPrefix(keyStr, contractsPrefix) {
			continue
		}
		name := strings.TrimPrefix(keyStr, contractsPrefix)
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		existingByName[name] = kv.Value
	}

	puts, dels, err := buildSnapshotSavePlan(bundleKey, existingByName, snapshots)
	if err != nil {
		return false, err
	}

	if len(puts)+len(dels) == 0 {
		return false, nil
	}

	var ops []clientv3.Op
	for i := range puts {
		p := &puts[i]
		ops = append(ops, clientv3.OpPut(p.key, p.val))
	}
	for _, k := range dels {
		ops = append(ops, clientv3.OpDelete(k))
	}

	if len(ops) <= maxSnapshotTxnOps {
		if _, err := r.client.Txn(ctx).Then(ops...).Commit(); err != nil {
			return false, apierrors.JoinStore("etcd txn save snapshots", err)
		}
		return true, nil
	}

	for i := 0; i < len(ops); i += maxSnapshotTxnOps {
		end := i + maxSnapshotTxnOps
		if end > len(ops) {
			end = len(ops)
		}
		chunk := ops[i:end]
		if _, err := r.client.Txn(ctx).Then(chunk...).Commit(); err != nil {
			return written, apierrors.JoinStore("etcd txn save snapshots (chunk)", err)
		}
		written = true
	}
	return true, nil
}

func (r *SnapshotRepository) GetSnapshots(ctx context.Context, bundleKey string) (snapshots []sharedgit.ContractSnapshot, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "GetSnapshots"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	prefix := fmt.Sprintf("%s%s/contracts/", snapshotPrefix, bundleKey)

	slog.Debug("Getting snapshots from etcd", "prefix", prefix)

	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, apierrors.JoinStore("get snapshots from etcd", err)
	}

	slog.Debug("Got response from etcd", "keys_count", len(resp.Kvs))

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

func (r *SnapshotRepository) ListBundleKeys(ctx context.Context) (out []string, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "ListBundleKeys"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	resp, err := r.client.Get(ctx, snapshotPrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, apierrors.JoinStore("list bundle keys from etcd", err)
	}

	bundleKeysMap := make(map[string]bool)
	for _, kv := range resp.Kvs {
		rel := strings.TrimPrefix(string(kv.Key), snapshotPrefix)
		bk, ok := bundleKeyFromSnapshotRelativeKey(rel)
		if !ok {
			continue
		}
		bundleKeysMap[bk] = true
	}

	out = make([]string, 0, len(bundleKeysMap))
	for bundleKey := range bundleKeysMap {
		out = append(out, bundleKey)
	}
	return out, nil
}

// bundleKeyFromSnapshotRelativeKey returns the bundle key from a key path relative to snapshotPrefix
// (e.g. "repo/remotes%2Forigin%2Fmaster/openapi/contracts/global.json").
func bundleKeyFromSnapshotRelativeKey(rel string) (string, bool) {
	idx := strings.Index(rel, "/contracts/")
	if idx <= 0 {
		return "", false
	}
	return rel[:idx], true
}
