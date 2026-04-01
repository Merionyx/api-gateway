package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"merionyx/api-gateway/internal/api-server/domain/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	controllerPrefix = "/api-gateway/api-server/controllers/"
)

type ControllerRepository struct {
	client *clientv3.Client
}

func NewControllerRepository(client *clientv3.Client) *ControllerRepository {
	return &ControllerRepository{
		client: client,
	}
}

type HeartbeatInfo struct {
	Timestamp time.Time `json:"timestamp"`
}

func (r *ControllerRepository) RegisterController(ctx context.Context, info models.ControllerInfo) error {
	slog.Info("Registering controller", "controller_id", info.ControllerID, "tenant", info.Tenant)

	key := fmt.Sprintf("%s%s", controllerPrefix, info.ControllerID)

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal controller info: %w", err)
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to register controller in etcd: %w", err)
	}

	heartbeatKey := fmt.Sprintf("%s%s/heartbeat", controllerPrefix, info.ControllerID)

	heartbeatData, err := json.Marshal(HeartbeatInfo{
		Timestamp: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat info: %w", err)
	}

	_, err = r.client.Put(ctx, heartbeatKey, string(heartbeatData))
	if err != nil {
		return fmt.Errorf("failed to save heartbeat: %w", err)
	}

	return nil
}

func (r *ControllerRepository) GetController(ctx context.Context, controllerID string) (*models.ControllerInfo, error) {
	key := fmt.Sprintf("%s%s", controllerPrefix, controllerID)

	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get controller from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("controller not found")
	}

	var info models.ControllerInfo
	if err := json.Unmarshal(resp.Kvs[0].Value, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal controller info: %w", err)
	}

	return &info, nil
}

func (r *ControllerRepository) ListControllers(ctx context.Context) ([]models.ControllerInfo, error) {
	resp, err := r.client.Get(ctx, controllerPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list controllers from etcd: %w", err)
	}

	var controllers []models.ControllerInfo
	for _, kv := range resp.Kvs {
		if string(kv.Key)[len(string(kv.Key))-10:] == "/heartbeat" {
			continue
		}

		var info models.ControllerInfo
		if err := json.Unmarshal(kv.Value, &info); err != nil {
			slog.Error("Failed to unmarshal controller info", "key", string(kv.Key), "error", err)
			continue
		}
		controllers = append(controllers, info)
	}

	return controllers, nil
}

func (r *ControllerRepository) UpdateControllerHeartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo) error {
	heartbeatKey := fmt.Sprintf("%s%s/heartbeat", controllerPrefix, controllerID)
	heartbeatData, err := json.Marshal(HeartbeatInfo{
		Timestamp: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat info: %w", err)
	}

	_, err = r.client.Put(ctx, heartbeatKey, string(heartbeatData))
	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	key := fmt.Sprintf("%s%s", controllerPrefix, controllerID)
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return fmt.Errorf("controller not found")
	}

	var info models.ControllerInfo
	if err := json.Unmarshal(resp.Kvs[0].Value, &info); err != nil {
		return fmt.Errorf("failed to unmarshal controller info: %w", err)
	}

	info.Environments = environments

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal controller info: %w", err)
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to update controller: %w", err)
	}

	return nil
}
