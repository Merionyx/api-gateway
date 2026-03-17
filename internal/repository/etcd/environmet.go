package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	environmentPrefix = "/api-gateway/environments/"
)

type environmentRepository struct {
	client *clientv3.Client
}

func NewEnvironmentRepository(client *clientv3.Client) interfaces.EnvironmentRepository {
	return &environmentRepository{
		client: client,
	}
}

// SaveEnvironment saves environment to etcd
func (r *environmentRepository) SaveEnvironment(ctx context.Context, env *models.Environment) error {
	key := environmentPrefix + env.Name + "/config"

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal environment: %w", err)
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to save environment to etcd: %w", err)
	}

	return nil
}

// GetEnvironment gets environment by name
func (r *environmentRepository) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	key := environmentPrefix + name + "/config"

	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("environment %s not found", name)
	}

	var env models.Environment
	if err := json.Unmarshal(resp.Kvs[0].Value, &env); err != nil {
		return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
	}

	for _, bundle := range env.Bundles.Static {
		fmt.Println("bundle", bundle.Name, bundle.Repository, bundle.Ref)
		safeRef := strings.ReplaceAll(bundle.Ref, "/", "%2F")
		prefix := fmt.Sprintf("%s%s/%s/contracts/", schemaPrefix, bundle.Repository, safeRef)
		fmt.Println("prefix", prefix)
		resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
		if err != nil {
			return nil, fmt.Errorf("failed to list contract snapshots from etcd: %w", err)
		}
		fmt.Println("resp", resp.Kvs)
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
		env.Snapshots = append(env.Snapshots, snapshots...)
	}

	return &env, nil
}

// ListEnvironments gets all environments
func (r *environmentRepository) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	resp, err := r.client.Get(ctx, environmentPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list environments from etcd: %w", err)
	}

	environments := make(map[string]*models.Environment)

	for _, kv := range resp.Kvs {
		if !strings.HasSuffix(string(kv.Key), "/config") {
			continue
		}

		var env models.Environment
		if err := json.Unmarshal(kv.Value, &env); err != nil {
			return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
		}

		for _, bundle := range env.Bundles.Static {
			safeRef := strings.ReplaceAll(bundle.Ref, "/", "%2F")
			prefix := fmt.Sprintf("%s%s/%s/contracts/", schemaPrefix, bundle.Repository, safeRef)
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
			env.Snapshots = append(env.Snapshots, snapshots...)
		}

		environments[env.Name] = &env
	}

	return environments, nil
}

// DeleteEnvironment deletes environment from etcd
func (r *environmentRepository) DeleteEnvironment(ctx context.Context, name string) error {
	key := environmentPrefix + name + "/config"
	_, err := r.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete environment from etcd: %w", err)
	}
	return nil
}

// WatchEnvironments creates watch channel for watching changes
func (r *environmentRepository) WatchEnvironments(ctx context.Context) clientv3.WatchChan {
	return r.client.Watch(ctx, environmentPrefix, clientv3.WithPrefix())
}
