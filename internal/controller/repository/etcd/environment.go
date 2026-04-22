package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const spanCtrlEtcdPkg = "internal/controller/repository/etcd"

// EnvironmentPrefix is the etcd prefix for environment config keys.
const EnvironmentPrefix = "/api-gateway/controller/environments/"

type environmentRepository struct {
	client *clientv3.Client
}

func NewEnvironmentRepository(client *clientv3.Client) interfaces.EnvironmentRepository {
	return &environmentRepository{
		client: client,
	}
}

// SaveEnvironment saves environment to etcd
func (r *environmentRepository) SaveEnvironment(ctx context.Context, env *models.Environment) (err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "SaveEnvironment"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	key := EnvironmentPrefix + env.Name + "/config"

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
func (r *environmentRepository) GetEnvironment(ctx context.Context, name string) (out *models.Environment, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "GetEnvironment"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	key := EnvironmentPrefix + name + "/config"

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

	return &env, nil
}

// ListEnvironments gets all environments
func (r *environmentRepository) ListEnvironments(ctx context.Context) (environments map[string]*models.Environment, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "ListEnvironments"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	resp, err := r.client.Get(ctx, EnvironmentPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list environments from etcd: %w", err)
	}

	environments = make(map[string]*models.Environment)
	for _, kv := range resp.Kvs {
		if !strings.HasSuffix(string(kv.Key), "/config") {
			continue
		}

		var env models.Environment
		if err := json.Unmarshal(kv.Value, &env); err != nil {
			return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
		}

		environments[env.Name] = &env
	}

	return environments, nil
}

// DeleteEnvironment deletes environment from etcd
func (r *environmentRepository) DeleteEnvironment(ctx context.Context, name string) (err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanCtrlEtcdPkg, "DeleteEnvironment"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	key := EnvironmentPrefix + name + "/config"
	_, err = r.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete environment from etcd: %w", err)
	}
	return nil
}

// WatchEnvironments creates watch channel for watching changes
func (r *environmentRepository) WatchEnvironments(ctx context.Context) clientv3.WatchChan {
	return r.client.Watch(ctx, EnvironmentPrefix, clientv3.WithPrefix())
}
