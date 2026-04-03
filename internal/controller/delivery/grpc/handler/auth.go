package handler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/shared/utils"
	authv1 "merionyx/api-gateway/pkg/api/auth/v1"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	environmentRepo                interfaces.EnvironmentRepository
	inMemoryEnvironmentsRepository interfaces.InMemoryEnvironmentsRepository
	schemaRepo                     interfaces.SchemaRepository
	etcdClient                     *clientv3.Client

	// Active connections
	subscribers map[string]chan *authv1.SyncAccessResponse
	mu          sync.RWMutex
}

func NewAuthHandler(
	environmentRepo interfaces.EnvironmentRepository,
	inMemoryEnvironmentsRepository interfaces.InMemoryEnvironmentsRepository,
	schemaRepo interfaces.SchemaRepository,
	etcdClient *clientv3.Client,
) *AuthHandler {
	handler := &AuthHandler{
		environmentRepo:                environmentRepo,
		inMemoryEnvironmentsRepository: inMemoryEnvironmentsRepository,
		schemaRepo:                     schemaRepo,
		etcdClient:                     etcdClient,
		subscribers:                    make(map[string]chan *authv1.SyncAccessResponse),
	}

	// Start the watcher for etcd
	go handler.watchEtcdChanges()

	return handler
}

// SyncAccess bidirectional streaming for synchronization
func (h *AuthHandler) SyncAccess(stream authv1.AuthService_SyncAccessServer) error {
	ctx := stream.Context()

	// Read the first request from the sidecar
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	sidecarID := req.SidecarId
	environment := req.Environment

	slog.Info("auth sync: new sidecar connection", "sidecar_id", sidecarID, "environment", environment)

	// Create a channel for this sidecar
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

	// Start the heartbeat
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Listen for updates and heartbeats
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case update := <-updateChan:
			// Send the update
			if err := stream.Send(update); err != nil {
				return err
			}

		case <-heartbeatTicker.C:
			// Send the heartbeat
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
func (h *AuthHandler) GetAccessConfig(ctx context.Context, req *authv1.GetAccessConfigRequest) (*authv1.AccessConfig, error) {
	return h.buildAccessConfig(ctx, req.Environment)
}

// buildAccessConfig builds the access configuration for the environment
func (h *AuthHandler) buildAccessConfig(ctx context.Context, environment string) (*authv1.AccessConfig, error) {
	var env *models.Environment

	config := &authv1.AccessConfig{
		Environment: environment,
		Contracts:   make([]*authv1.ContractAccess, 0),
		Version:     time.Now().Unix(),
	}

	// Get the environment from etcd
	env, err := h.environmentRepo.GetEnvironment(ctx, environment)
	if err != nil {
		env, err = h.inMemoryEnvironmentsRepository.GetEnvironment(ctx, environment)

		if err != nil {
			return config, fmt.Errorf("environment not found: %w", err)
		}

		// For each snapshot get the contracts
		for _, snapshot := range env.Snapshots {
			contractAccess := &authv1.ContractAccess{
				ContractName: snapshot.Name,
				Prefix:       snapshot.Prefix,
				Secure:       snapshot.Access.Secure,
				Apps:         make([]*authv1.AppAccess, 0),
			}

			// Filter applications by environment
			for _, app := range snapshot.Access.Apps {
				// Check if there is access for this environment
				hasAccess := false
				if len(app.Environments) == 0 {
					// If environments are not specified - access everywhere
					hasAccess = true
				} else {
					for _, env := range app.Environments {
						if env == environment {
							hasAccess = true
							break
						}
					}
				}

				if hasAccess {
					contractAccess.Apps = append(contractAccess.Apps, &authv1.AppAccess{
						AppId:        app.AppID,
						Environments: app.Environments,
					})
				}
			}

			config.Contracts = append(config.Contracts, contractAccess)
		}
	}

	// For each bundle get the contracts
	for _, bundle := range env.Bundles.Static {
		// Get all contracts from the bundle
		snapshots, err := h.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			slog.Warn("auth sync: failed to list snapshots for bundle", "bundle", bundle.Name, "error", err)
			continue
		}

		// Convert to ContractAccess
		for _, snapshot := range snapshots {
			contractAccess := &authv1.ContractAccess{
				ContractName: snapshot.Name,
				Prefix:       snapshot.Prefix,
				Secure:       snapshot.Access.Secure,
				Apps:         make([]*authv1.AppAccess, 0),
			}

			// Filter applications by environment
			for _, app := range snapshot.Access.Apps {
				// Check if there is access for this environment
				hasAccess := false
				if len(app.Environments) == 0 {
					// If environments are not specified - access everywhere
					hasAccess = true
				} else {
					for _, envPattern := range app.Environments {
						if utils.MatchesEnvironmentPattern(environment, envPattern) {
							hasAccess = true
							break
						}
					}
				}

				if hasAccess {
					contractAccess.Apps = append(contractAccess.Apps, &authv1.AppAccess{
						AppId:        app.AppID,
						Environments: app.Environments,
					})
				}
			}

			config.Contracts = append(config.Contracts, contractAccess)
		}
	}

	return config, nil
}

// watchEtcdChanges watches for changes in etcd and notifies sidecars
func (h *AuthHandler) watchEtcdChanges() {
	watchChan := h.etcdClient.Watch(context.Background(), "/api-gateway/controller/schemas/", clientv3.WithPrefix())

	slog.Info("auth sync: watching etcd for schema changes")

	for watchResp := range watchChan {
		if watchResp.Err() != nil {
			slog.Warn("auth sync: etcd watch error", "error", watchResp.Err())
			continue
		}

		for _, event := range watchResp.Events {
			// Parse the key to determine the environment
			// /api-gateway/schemas/{repo}/{ref}/contracts/{contract}/snapshot

			slog.Info("auth sync: schema key changed", "key", string(event.Kv.Key))

			// Notify all subscribers
			// TODO: Determine which environments are affected
			// For simplicity - notify all
			h.notifyAllSubscribers()
		}
	}
}

// notifyAllSubscribers notifies all connected sidecars
func (h *AuthHandler) notifyAllSubscribers() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for subscriberKey, updateChan := range h.subscribers {
		// Extract the environment from the key
		// subscriberKey format: "dev:sidecar-123"
		parts := splitSubscriberKey(subscriberKey)
		environment := parts[0]

		// Build the updated configuration
		config, err := h.buildAccessConfig(context.Background(), environment)
		if err != nil {
			slog.Warn("auth sync: failed to build config for subscriber", "subscriber", subscriberKey, "error", err)
			continue
		}

		// Send the update (non-blocking)
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
	// "dev:sidecar-123" -> ["dev", "sidecar-123"]
	for i, c := range key {
		if c == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key, ""}
}
