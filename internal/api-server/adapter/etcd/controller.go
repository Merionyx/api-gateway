package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

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

func validateControllerInfo(info models.ControllerInfo) error {
	if err := models.ValidateControllerServiceScopes(info); err != nil {
		return fmt.Errorf("%w: %s", apierrors.ErrInvalidInput, err.Error())
	}
	return nil
}

func parseControllerInfoValue(raw []byte) (models.ControllerInfo, error) {
	var parsed models.ControllerInfo
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return models.ControllerInfo{}, fmt.Errorf("unmarshal controller info: %w", err)
	}
	if err := validateControllerInfo(parsed); err != nil {
		return models.ControllerInfo{}, err
	}
	return parsed, nil
}

func (r *ControllerRepository) RegisterController(ctx context.Context, info models.ControllerInfo) (err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "RegisterController"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	slog.Info("Registering controller", "controller_id", info.ControllerID, "tenant", info.Tenant)

	key := fmt.Sprintf("%s%s", controllerPrefix, info.ControllerID)

	info.Environments = models.CanonicalEnvironmentsForStorage(info.Environments)
	if err := validateControllerInfo(info); err != nil {
		return err
	}

	data, err := json.Marshal(info)
	if err != nil {
		return apierrors.JoinStore("marshal controller info", err)
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return apierrors.JoinStore("register controller in etcd", err)
	}

	heartbeatKey := fmt.Sprintf("%s%s/heartbeat", controllerPrefix, info.ControllerID)

	heartbeatData, err := json.Marshal(HeartbeatInfo{
		Timestamp: time.Now(),
	})
	if err != nil {
		return apierrors.JoinStore("marshal heartbeat info", err)
	}

	_, err = r.client.Put(ctx, heartbeatKey, string(heartbeatData))
	if err != nil {
		return apierrors.JoinStore("save controller heartbeat", err)
	}

	return nil
}

func (r *ControllerRepository) GetController(ctx context.Context, controllerID string) (info *models.ControllerInfo, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "GetController"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	key := fmt.Sprintf("%s%s", controllerPrefix, controllerID)

	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, apierrors.JoinStore("get controller from etcd", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("%w", apierrors.ErrNotFound)
	}

	parsed, err := parseControllerInfoValue(resp.Kvs[0].Value)
	if err != nil {
		return nil, apierrors.JoinStore("parse controller info", err)
	}

	return &parsed, nil
}

func (r *ControllerRepository) ListControllers(ctx context.Context) (out []models.ControllerInfo, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "ListControllers"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	resp, err := r.client.Get(ctx, controllerPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, apierrors.JoinStore("list controllers from etcd", err)
	}

	for _, kv := range resp.Kvs {
		if string(kv.Key)[len(string(kv.Key))-10:] == "/heartbeat" {
			continue
		}

		row, err := parseControllerInfoValue(kv.Value)
		if err != nil {
			return nil, apierrors.JoinStore("parse controller info", fmt.Errorf("key %q: %w", string(kv.Key), err))
		}
		out = append(out, row)
	}

	return out, nil
}

func (r *ControllerRepository) GetHeartbeat(ctx context.Context, controllerID string) (t time.Time, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "GetHeartbeat"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	mainKey := fmt.Sprintf("%s%s", controllerPrefix, controllerID)
	resp, err := r.client.Get(ctx, mainKey)
	if err != nil {
		return time.Time{}, apierrors.JoinStore("get controller from etcd", err)
	}
	if len(resp.Kvs) == 0 {
		return time.Time{}, fmt.Errorf("%w", apierrors.ErrNotFound)
	}

	heartbeatKey := fmt.Sprintf("%s%s/heartbeat", controllerPrefix, controllerID)
	hbResp, err := r.client.Get(ctx, heartbeatKey)
	if err != nil {
		return time.Time{}, apierrors.JoinStore("get heartbeat from etcd", err)
	}
	if len(hbResp.Kvs) == 0 {
		return time.Time{}, fmt.Errorf("%w", apierrors.ErrNotFound)
	}

	var hi HeartbeatInfo
	if err := json.Unmarshal(hbResp.Kvs[0].Value, &hi); err != nil {
		return time.Time{}, apierrors.JoinStore("unmarshal heartbeat", err)
	}
	if hi.Timestamp.IsZero() {
		return time.Time{}, fmt.Errorf("%w", apierrors.ErrNotFound)
	}
	return hi.Timestamp, nil
}

func (r *ControllerRepository) UpdateControllerHeartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo, registryPayloadVersion int32) (mainKeyUpdated bool, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIEtcdPkg, "UpdateControllerHeartbeat"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	heartbeatKey := fmt.Sprintf("%s%s/heartbeat", controllerPrefix, controllerID)
	heartbeatData, err := json.Marshal(HeartbeatInfo{
		Timestamp: time.Now(),
	})
	if err != nil {
		return false, apierrors.JoinStore("marshal heartbeat info", err)
	}

	_, err = r.client.Put(ctx, heartbeatKey, string(heartbeatData))
	if err != nil {
		return false, apierrors.JoinStore("update heartbeat", err)
	}

	key := fmt.Sprintf("%s%s", controllerPrefix, controllerID)
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return false, apierrors.JoinStore("get controller for heartbeat update", err)
	}

	if len(resp.Kvs) == 0 {
		return false, fmt.Errorf("%w", apierrors.ErrNotFound)
	}

	info, err := parseControllerInfoValue(resp.Kvs[0].Value)
	if err != nil {
		return false, apierrors.JoinStore("parse controller info", err)
	}

	info.Environments = models.CanonicalEnvironmentsForStorage(environments)
	if registryPayloadVersion > 0 {
		info.RegistryPayloadVersion = registryPayloadVersion
	}
	if err := validateControllerInfo(info); err != nil {
		return false, err
	}

	data, err := json.Marshal(info)
	if err != nil {
		return false, apierrors.JoinStore("marshal controller info", err)
	}

	prev := resp.Kvs[0].Value
	if string(prev) == string(data) {
		slog.Debug("API Server etcd: controller record unchanged, skip write", "controller_id", controllerID)
		return false, nil
	}

	_, err = r.client.Put(ctx, key, string(data))
	if err != nil {
		return false, apierrors.JoinStore("update controller", err)
	}

	return true, nil
}
