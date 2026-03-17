package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/repository/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	schemaPrefix = "/api-gateway/schemas/"
)

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
func (r *schemaRepository) SaveContractSnapshot(ctx context.Context, repo, ref, contract string, snapshot *git.ContractSnapshot) error {
	key := r.buildContractKey(repo, ref, contract)

	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal contract snapshot: %w", err)
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to save contract snapshot to etcd: %w", err)
	}

	return nil
}

// GetContractSnapshot gets contract snapshot from etcd
func (r *schemaRepository) GetContractSnapshot(ctx context.Context, repo, ref, contract string) (*git.ContractSnapshot, error) {
	key := r.buildContractKey(repo, ref, contract)

	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract snapshot from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("contract snapshot not found: %s/%s/%s", repo, ref, contract)
	}

	var snapshot git.ContractSnapshot
	if err := json.Unmarshal(resp.Kvs[0].Value, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract snapshot: %w", err)
	}

	return &snapshot, nil
}

// GetEnvironmentSnapshots gets all snapshots for environment
func (r *schemaRepository) GetEnvironmentSnapshots(ctx context.Context, envName string) ([]git.ContractSnapshot, error) {
	prefix := schemaPrefix

	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots from etcd: %w", err)
	}

	var snapshots []git.ContractSnapshot

	for _, kv := range resp.Kvs {
		if !strings.HasSuffix(string(kv.Key), "/snapshot") {
			continue
		}

		var snapshot git.ContractSnapshot
		if err := json.Unmarshal(kv.Value, &snapshot); err != nil {
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// ListContractSnapshots lists all contract snapshots for repository/ref
func (r *schemaRepository) ListContractSnapshots(ctx context.Context, repo, ref string) ([]git.ContractSnapshot, error) {
	safeRef := strings.ReplaceAll(ref, "/", "%2F")
	prefix := fmt.Sprintf("%s%s/%s/contracts/", schemaPrefix, repo, safeRef)
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list contract snapshots from etcd: %w", err)
	}
	var snapshots []git.ContractSnapshot
	for _, kv := range resp.Kvs {
		if !strings.HasSuffix(string(kv.Key), "/snapshot") {
			continue
		}
		var snapshot git.ContractSnapshot
		if err := json.Unmarshal(kv.Value, &snapshot); err != nil {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// buildContractKey builds key for contract in etcd
func (r *schemaRepository) buildContractKey(repo, ref, contract string) string {
	safeRef := strings.ReplaceAll(ref, "/", "%2F")
	return fmt.Sprintf("%s%s/%s/contracts/%s/snapshot", schemaPrefix, repo, safeRef, contract)
}
