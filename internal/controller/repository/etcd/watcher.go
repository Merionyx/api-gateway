package etcd

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"merionyx/api-gateway/internal/controller/domain/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// WatcherCallback callback function for handling changes
type WatcherCallback func(eventType string, env *models.Environment) error

// StartEnvironmentWatcher starts watcher for watching changes of environments
func StartEnvironmentWatcher(ctx context.Context, client *clientv3.Client, callback WatcherCallback) {
	watchChan := client.Watch(ctx, environmentPrefix, clientv3.WithPrefix())

	slog.Info("environment watcher started")

	for watchResp := range watchChan {
		if watchResp.Err() != nil {
			slog.Warn("environment watch error", "error", watchResp.Err())
			continue
		}

		for _, event := range watchResp.Events {
			if !strings.HasSuffix(string(event.Kv.Key), "/config") {
				continue
			}

			switch event.Type {
			case clientv3.EventTypePut:
				var env models.Environment
				if err := json.Unmarshal(event.Kv.Value, &env); err != nil {
					slog.Warn("failed to unmarshal environment", "error", err)
					continue
				}

				if err := callback("put", &env); err != nil {
					slog.Warn("failed to handle environment update", "error", err)
				}

			case clientv3.EventTypeDelete:
				envName := extractEnvNameFromKey(string(event.Kv.Key))
				env := &models.Environment{Name: envName}

				if err := callback("delete", env); err != nil {
					slog.Warn("failed to handle environment deletion", "error", err)
				}
			}
		}
	}

	slog.Info("environment watcher stopped")
}

// extractEnvNameFromKey extracts environment name from etcd key
// Example: /api-gateway/environments/dev/config -> dev
func extractEnvNameFromKey(key string) string {
	withoutPrefix := strings.TrimPrefix(key, environmentPrefix)
	parts := strings.Split(withoutPrefix, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
