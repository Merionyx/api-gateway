package etcd

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"merionyx/api-gateway/internal/controller/domain/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// WatcherCallback callback function for handling changes
type WatcherCallback func(eventType string, env *models.Environment) error

// StartEnvironmentWatcher starts watcher for watching changes of environments
func StartEnvironmentWatcher(ctx context.Context, client *clientv3.Client, callback WatcherCallback) {
	watchChan := client.Watch(ctx, environmentPrefix, clientv3.WithPrefix())

	log.Println("Environment watcher started")

	for watchResp := range watchChan {
		if watchResp.Err() != nil {
			log.Printf("Watch error: %v", watchResp.Err())
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
					log.Printf("Failed to unmarshal environment: %v", err)
					continue
				}

				if err := callback("put", &env); err != nil {
					log.Printf("Failed to handle environment update: %v", err)
				}

			case clientv3.EventTypeDelete:
				envName := extractEnvNameFromKey(string(event.Kv.Key))
				env := &models.Environment{Name: envName}

				if err := callback("delete", env); err != nil {
					log.Printf("Failed to handle environment deletion: %v", err)
				}
			}
		}
	}

	log.Println("Environment watcher stopped")
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
