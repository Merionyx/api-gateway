package handler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	authv1 "merionyx/api-gateway/pkg/api/auth/v1"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	environmentRepo                interfaces.EnvironmentRepository
	inMemoryEnvironmentsRepository interfaces.InMemoryEnvironmentsRepository
	schemaRepo                     interfaces.SchemaRepository
	etcdClient                     *clientv3.Client

	// Активные подключения
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

	// Запускаем watcher для etcd
	go handler.watchEtcdChanges()

	return handler
}

// SyncAccess bidirectional streaming для синхронизации
func (h *AuthHandler) SyncAccess(stream authv1.AuthService_SyncAccessServer) error {
	ctx := stream.Context()

	// Читаем первый запрос от sidecar
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	sidecarID := req.SidecarId
	environment := req.Environment

	log.Printf("[AUTH-SYNC] New connection from sidecar %s (env: %s)", sidecarID, environment)

	// Создаем канал для этого sidecar
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

	// Отправляем начальную конфигурацию
	initialConfig, err := h.buildAccessConfig(ctx, environment)
	if err != nil {
		return fmt.Errorf("failed to build initial config: %w", err)
	}

	if err := stream.Send(&authv1.SyncAccessResponse{
		Message: &authv1.SyncAccessResponse_InitialConfig{
			InitialConfig: initialConfig,
		},
	}); err != nil {
		return err
	}

	log.Printf("[AUTH-SYNC] Sent initial config to %s: %d contracts", sidecarID, len(initialConfig.Contracts))

	// Запускаем heartbeat
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Слушаем обновления и heartbeats
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case update := <-updateChan:
			// Отправляем обновление
			if err := stream.Send(update); err != nil {
				return err
			}

		case <-heartbeatTicker.C:
			// Отправляем heartbeat
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

// GetAccessConfig получить текущую конфигурацию (unary)
func (h *AuthHandler) GetAccessConfig(ctx context.Context, req *authv1.GetAccessConfigRequest) (*authv1.AccessConfig, error) {
	return h.buildAccessConfig(ctx, req.Environment)
}

// buildAccessConfig строит конфигурацию доступа для окружения
func (h *AuthHandler) buildAccessConfig(ctx context.Context, environment string) (*authv1.AccessConfig, error) {
	// Получаем environment из etcd
	env, err := h.environmentRepo.GetEnvironment(ctx, environment)
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}

	config := &authv1.AccessConfig{
		Environment: environment,
		Contracts:   make([]*authv1.ContractAccess, 0),
		Version:     time.Now().Unix(),
	}

	// Для каждого bundle получаем контракты
	for _, bundle := range env.Bundles.Static {
		// Получаем все контракты из bundle
		snapshots, err := h.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref)
		if err != nil {
			log.Printf("Failed to get snapshots for bundle %s: %v", bundle.Name, err)
			continue
		}

		// Конвертируем в ContractAccess
		for _, snapshot := range snapshots {
			contractAccess := &authv1.ContractAccess{
				ContractName: snapshot.Name,
				Secure:       snapshot.Access.Secure,
				Apps:         make([]*authv1.AppAccess, 0),
			}

			// Фильтруем приложения по environment
			for _, app := range snapshot.Access.Apps {
				// Проверяем, есть ли доступ для этого environment
				hasAccess := false
				if len(app.Environments) == 0 {
					// Если environments не указаны - доступ везде
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

// watchEtcdChanges следит за изменениями в etcd и уведомляет sidecars
func (h *AuthHandler) watchEtcdChanges() {
	watchChan := h.etcdClient.Watch(context.Background(), "/api-gateway/schemas/", clientv3.WithPrefix())

	log.Println("[AUTH-SYNC] Started watching etcd for schema changes")

	for watchResp := range watchChan {
		if watchResp.Err() != nil {
			log.Printf("[AUTH-SYNC] Watch error: %v", watchResp.Err())
			continue
		}

		for _, event := range watchResp.Events {
			// Парсим ключ для определения environment
			// /api-gateway/schemas/{repo}/{ref}/contracts/{contract}/snapshot

			log.Printf("[AUTH-SYNC] Schema changed: %s", string(event.Kv.Key))

			// Уведомляем всех подписчиков
			// TODO: Определить какие environments затронуты
			// Для упрощения - уведомляем всех
			h.notifyAllSubscribers()
		}
	}
}

// notifyAllSubscribers уведомляет всех подключенных sidecars
func (h *AuthHandler) notifyAllSubscribers() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for subscriberKey, updateChan := range h.subscribers {
		// Извлекаем environment из ключа
		// subscriberKey формат: "dev:sidecar-123"
		parts := splitSubscriberKey(subscriberKey)
		environment := parts[0]

		// Строим обновленную конфигурацию
		config, err := h.buildAccessConfig(context.Background(), environment)
		if err != nil {
			log.Printf("[AUTH-SYNC] Failed to build config for %s: %v", subscriberKey, err)
			continue
		}

		// Отправляем обновление (non-blocking)
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
