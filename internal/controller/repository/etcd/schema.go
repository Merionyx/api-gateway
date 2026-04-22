package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ControllerWatchPrefix is the etcd prefix for controller-owned keys (schemas, environments, etc.).
const ControllerWatchPrefix = "/api-gateway/controller/"

// SchemaPrefix is the etcd prefix for contract snapshot keys (exported for cache/watch parsing).
const SchemaPrefix = "/api-gateway/controller/schemas/"

type schemaRepository struct {
	client *clientv3.Client
}

// NewSchemaRepository creates new repository for working with schemas
func NewSchemaRepository(client *clientv3.Client) interfaces.SchemaRepository {
	return &schemaRepository{
		client: client,
	}
}

// SaveContractSnapshot saves contract snapshot to etcd
func (r *schemaRepository) SaveContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string, snapshot *models.ContractSnapshot) (err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "SaveContractSnapshot"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	key := r.buildContractKey(repo, ref, bundlePath, contract)

	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal contract snapshot: %w", err)
	}

	getResp, err := r.client.Get(ctx, key)
	if err == nil && len(getResp.Kvs) == 1 && string(getResp.Kvs[0].Value) == string(data) {
		slog.Debug("Controller etcd: contract snapshot unchanged, skip write", "key", key, "contract", contract)
		return nil
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to save contract snapshot to etcd: %w", err)
	}

	slog.Info("Controller etcd: contract snapshot saved", "key", key, "contract", contract)
	return nil
}

// GetContractSnapshot gets contract snapshot from etcd
func (r *schemaRepository) GetContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string) (snap *models.ContractSnapshot, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "GetContractSnapshot"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	key := r.buildContractKey(repo, ref, bundlePath, contract)

	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract snapshot from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("contract snapshot not found: %s/%s/%s/%s", repo, ref, bundlePath, contract)
	}

	var parsed models.ContractSnapshot
	if err := json.Unmarshal(resp.Kvs[0].Value, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract snapshot: %w", err)
	}

	return &parsed, nil
}

// GetEnvironmentSnapshots gets all snapshots for environment
func (r *schemaRepository) GetEnvironmentSnapshots(ctx context.Context, envName string) (out []models.ContractSnapshot, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "GetEnvironmentSnapshots"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	resp, err := r.client.Get(ctx, SchemaPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots from etcd: %w", err)
	}

	for _, kv := range resp.Kvs {
		if !strings.HasSuffix(string(kv.Key), "/snapshot") {
			continue
		}

		var row models.ContractSnapshot
		if err := json.Unmarshal(kv.Value, &row); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out, nil
}

// ListContractSnapshots lists all contract snapshots for repository/ref/path
func (r *schemaRepository) ListContractSnapshots(ctx context.Context, repo, ref, bundlePath string) (out []models.ContractSnapshot, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "ListContractSnapshots"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	prefix := r.contractsPrefix(repo, ref, bundlePath)
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list contract snapshots from etcd: %w", err)
	}
	for _, kv := range resp.Kvs {
		if !strings.HasSuffix(string(kv.Key), "/snapshot") {
			continue
		}
		var row models.ContractSnapshot
		if err := json.Unmarshal(kv.Value, &row); err != nil {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (r *schemaRepository) contractsPrefix(repo, ref, bundlePath string) string {
	return fmt.Sprintf("%s%s/%s/%s/contracts/", SchemaPrefix, repo, bundlekey.EscapeRef(ref), bundlekey.EscapePath(bundlePath))
}

func (r *schemaRepository) buildContractKey(repo, ref, bundlePath, contract string) string {
	return fmt.Sprintf("%s%s/snapshot", r.contractsPrefix(repo, ref, bundlePath), contract)
}

// WatchContractBundlesSnapshots watches contract bundles snapshots for repository/ref
func (r *schemaRepository) WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan {
	return r.client.Watch(ctx, SchemaPrefix, clientv3.WithPrefix())
}
