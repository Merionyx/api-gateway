package handler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	authv1 "github.com/merionyx/api-gateway/pkg/grpc/auth/v1"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/controller/repository/cache"
	"github.com/merionyx/api-gateway/internal/controller/repository/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// AuthHandler — gRPC: stream SyncAccess, watch etcd, нотификация подписчиков. Сборка пейлоада — AuthConfigBuilder.
type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	configBuilder  *AuthConfigBuilder
	schemaCache    *cache.SchemaCache
	bundleIndex    *bundleenv.Index
	metricsEnabled bool
	etcdClient     *clientv3.Client

	subscribers map[string]chan *authv1.SyncAccessResponse
	mu          sync.RWMutex
}

func NewAuthHandler(
	environmentRepo interfaces.EnvironmentRepository,
	inMemoryEnvironmentsRepository interfaces.InMemoryEnvironmentsRepository,
	schemaRepo interfaces.SchemaRepository,
	schemaCache *cache.SchemaCache,
	bundleIndex *bundleenv.Index,
	metricsEnabled bool,
	etcdClient *clientv3.Client,
) *AuthHandler {
	cfg := NewAuthConfigBuilder(environmentRepo, inMemoryEnvironmentsRepository, schemaRepo)
	handler := &AuthHandler{
		configBuilder:  cfg,
		schemaCache:    schemaCache,
		bundleIndex:    bundleIndex,
		metricsEnabled: metricsEnabled,
		etcdClient:     etcdClient,
		subscribers:      make(map[string]chan *authv1.SyncAccessResponse),
	}

	go handler.watchEtcdChanges()
	if bundleIndex != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			bundleIndex.Rebuild(ctx)
			ctrlmetrics.RecordBundleEnvIndexRebuild(metricsEnabled)
		}()
	}

	return handler
}

// SyncAccess bidirectional streaming for synchronization
func (h *AuthHandler) SyncAccess(stream authv1.AuthService_SyncAccessServer) error {
	ctx := stream.Context()

	req, err := stream.Recv()
	if err != nil {
		return err
	}

	sidecarID := req.SidecarId
	environment := req.Environment

	slog.Info("auth sync: new sidecar connection", "sidecar_id", sidecarID, "environment", environment)

	updateChan := make(chan *authv1.SyncAccessResponse, 100)

	h.mu.Lock()
	subscriberKey := fmt.Sprintf("%s:%s", environment, sidecarID)
	h.subscribers[subscriberKey] = updateChan
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.subscribers, subscriberKey)
		close(updateChan)
		h.mu.Unlock()
		slog.Info("auth sync: sidecar disconnected", "sidecar_id", sidecarID)
	}()

	initialConfig, err := h.buildAccessConfig(ctx, environment)
	if err == nil {
		slog.Debug("auth sync: initial config built", "environment", environment, "contracts", len(initialConfig.Contracts))
	}
	if err != nil {
		slog.Warn("auth sync: environment not found, sending empty config", "environment", environment, "error", err)
		initialConfig = &authv1.AccessConfig{
			Environment: environment,
			Contracts:   make([]*authv1.ContractAccess, 0),
			Version:     time.Now().Unix(),
		}
	}
	if err := stream.Send(&authv1.SyncAccessResponse{
		Message: &authv1.SyncAccessResponse_InitialConfig{
			InitialConfig: initialConfig,
		},
	}); err != nil {
		return err
	}

	slog.Info("auth sync: sent initial config", "sidecar_id", sidecarID, "contracts", len(initialConfig.Contracts))

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case update := <-updateChan:
			if err := stream.Send(update); err != nil {
				return err
			}

		case <-heartbeatTicker.C:
			if err := stream.Send(&authv1.SyncAccessResponse{
				Message: &authv1.SyncAccessResponse_Heartbeat{
					Heartbeat: &authv1.Heartbeat{
						Timestamp: time.Now().Unix(),
					},
				},
			}); err != nil {
				return err
			}
		}
	}
}

// GetAccessConfig get the current configuration (unary)
func (h *AuthHandler) GetAccessConfig(ctx context.Context, req *authv1.GetAccessConfigRequest) (*authv1.GetAccessConfigResponse, error) {
	cfg, err := h.buildAccessConfig(ctx, req.Environment)
	if err != nil {
		return nil, err
	}
	return &authv1.GetAccessConfigResponse{Config: cfg}, nil
}

func (h *AuthHandler) buildAccessConfig(ctx context.Context, environment string) (*authv1.AccessConfig, error) {
	start := time.Now()
	defer func() {
		ctrlmetrics.ObserveAuthBuildAccessConfig(h.metricsEnabled, time.Since(start))
	}()
	return h.configBuilder.BuildAccessConfig(ctx, environment)
}

func (h *AuthHandler) watchEtcdChanges() {
	watchChan := h.etcdClient.Watch(context.Background(), etcd.ControllerWatchPrefix, clientv3.WithPrefix())

	slog.Info("auth sync: watching etcd for schema changes")

	for watchResp := range watchChan {
		if watchResp.Err() != nil {
			slog.Warn("auth sync: etcd watch error", "error", watchResp.Err())
			continue
		}

		for _, event := range watchResp.Events {
			key := string(event.Kv.Key)
			slog.Info("auth sync: controller key changed", "key", key)

			eff := cache.ClassifyControllerEtcdWatchKey(key)
			if eff.Ignore {
				continue
			}

			if eff.SchemaBundleKey != "" {
				if h.schemaCache != nil {
					h.schemaCache.InvalidateBundleKey(eff.SchemaBundleKey)
				}
				plan := PlanAuthSchemaNotify(eff.SchemaBundleKey, h.bundleIndex, h.metricsEnabled)
				if plan.NotifyAll {
					slog.Warn("auth sync: no environments mapped to bundle; notifying all subscribers", "bundle_key", eff.SchemaBundleKey)
					h.notifyAllSubscribers()
					continue
				}
				h.notifyEnvironments(plan.TargetEnvironments)
				continue
			}

			if eff.Environment != "" {
				if h.bundleIndex != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					h.bundleIndex.Rebuild(ctx)
					cancel()
					ctrlmetrics.RecordBundleEnvIndexRebuild(h.metricsEnabled)
				}
				h.notifyEnvironments([]string{eff.Environment})
			}
		}
	}
}

func (h *AuthHandler) notifyEnvironments(envs []string) {
	if len(envs) == 0 {
		return
	}
	want := make(map[string]struct{}, len(envs))
	for _, e := range envs {
		want[e] = struct{}{}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for subscriberKey, updateChan := range h.subscribers {
		environment := splitSubscriberKey(subscriberKey)[0]
		if _, ok := want[environment]; !ok {
			continue
		}

		config, err := h.buildAccessConfig(context.Background(), environment)
		if err != nil {
			slog.Warn("auth sync: failed to build config for subscriber", "subscriber", subscriberKey, "error", err)
			continue
		}

		select {
		case updateChan <- &authv1.SyncAccessResponse{
			Message: &authv1.SyncAccessResponse_InitialConfig{
				InitialConfig: config,
			},
		}:
			slog.Debug("auth sync: sent update to subscriber", "subscriber", subscriberKey)
		default:
			slog.Warn("auth sync: update channel full, skipping", "subscriber", subscriberKey)
		}
	}
}

func (h *AuthHandler) notifyAllSubscribers() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for subscriberKey, updateChan := range h.subscribers {
		environment := splitSubscriberKey(subscriberKey)[0]

		config, err := h.buildAccessConfig(context.Background(), environment)
		if err != nil {
			slog.Warn("auth sync: failed to build config for subscriber", "subscriber", subscriberKey, "error", err)
			continue
		}

		select {
		case updateChan <- &authv1.SyncAccessResponse{
			Message: &authv1.SyncAccessResponse_InitialConfig{
				InitialConfig: config,
			},
		}:
			slog.Debug("auth sync: sent update to subscriber", "subscriber", subscriberKey)
		default:
			slog.Warn("auth sync: update channel full, skipping", "subscriber", subscriberKey)
		}
	}
}

func splitSubscriberKey(key string) []string {
	for i, c := range key {
		if c == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key, ""}
}
