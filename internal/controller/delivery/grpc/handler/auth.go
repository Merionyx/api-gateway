package handler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
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

	log.Printf("[AUTH-SYNC] New connection from sidecar %s (env: %s)", sidecarID, environment)

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
		log.Printf("[AUTH-SYNC] Disconnected sidecar %s", sidecarID)
	}()

	initialConfig, err := h.buildAccessConfig(ctx, environment)
	log.Printf("[AUTH-SYNC] Initial config: %v", initialConfig)
	if err != nil {
		log.Printf("[AUTH-SYNC] Environment %s not found, sending empty config: %v", environment, err)
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

	log.Printf("[AUTH-SYNC] Sent initial config to %s: %d contracts", sidecarID, len(initialConfig.Contracts))

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

		// For each snapshot get the contracts
		for _, snapshot := range env.Snapshots {
			contractAccess := &authv1.ContractAccess{
				ContractName: snapshot.Name,
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
		snapshots, err := h.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref)
		if err != nil {
			log.Printf("Failed to get snapshots for bundle %s: %v", bundle.Name, err)
			continue
		}

		// Convert to ContractAccess
		for _, snapshot := range snapshots {
			contractAccess := &authv1.ContractAccess{
				ContractName: snapshot.Name,
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

	return config, nil
}

// watchEtcdChanges watches for changes in etcd and notifies sidecars
func (h *AuthHandler) watchEtcdChanges() {
	watchChan := h.etcdClient.Watch(context.Background(), "/api-gateway/schemas/", clientv3.WithPrefix())

	log.Println("[AUTH-SYNC] Started watching etcd for schema changes")

	for watchResp := range watchChan {
		if watchResp.Err() != nil {
			log.Printf("[AUTH-SYNC] Watch error: %v", watchResp.Err())
			continue
		}

		for _, event := range watchResp.Events {
			// Parse the key to determine the environment
			// /api-gateway/schemas/{repo}/{ref}/contracts/{contract}/snapshot

			log.Printf("[AUTH-SYNC] Schema changed: %s", string(event.Kv.Key))

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
			log.Printf("[AUTH-SYNC] Failed to build config for %s: %v", subscriberKey, err)
			continue
		}

		// Send the update (non-blocking)
		select {
		case updateChan <- &authv1.SyncAccessResponse{
			Message: &authv1.SyncAccessResponse_InitialConfig{
				InitialConfig: config,
			},
		}:
			log.Printf("[AUTH-SYNC] Sent update to %s", subscriberKey)
		default:
			log.Printf("[AUTH-SYNC] Channel full for %s, skipping update", subscriberKey)
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
