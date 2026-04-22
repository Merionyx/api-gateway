package usecase

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	"github.com/merionyx/api-gateway/internal/controller/repository/cache"
	ctrlrepoetcd "github.com/merionyx/api-gateway/internal/controller/repository/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// APIServerSyncUseCase keeps the Gateway Controller in sync with API Server (leader stream) and
// reconciles xDS from controller-local etcd on every replica (follower watch). Internally it
// composes a registry DTO builder, gRPC leader stream, and etcd follower watch.
// Деградация (частичные списки имён, skip env, read materialized): RegistryEnvironmentsBuildReport, счётчик
// gateway_controller_registry_environments_build_warnings_total и агрегированные логи (P.5 бэклога).
type APIServerSyncUseCase struct {
	config *config.Config

	registry   *registryEnvironmentsBuilder
	leader     *leaderAPIServerStream
	etcdFollow *etcdFollowerWatch
}

func NewAPIServerSyncUseCase(
	cfg *config.Config,
	schemaRepo interfaces.SchemaRepository,
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository,
	environmentRepo interfaces.EnvironmentRepository,
	etcdClient *clientv3.Client,
	bundleEnvIndex *bundleenv.Index,
	schemaCache *cache.SchemaCache,
	materialized *ctrlrepoetcd.MaterializedStore,
	effectiveReconciler interfaces.EffectiveReconciler,
) *APIServerSyncUseCase {
	controllerID := strings.TrimSpace(cfg.HA.ControllerID)
	if controllerID == "" {
		var err error
		controllerID, err = os.Hostname()
		if err != nil {
			slog.Error("Failed to get hostname", "error", err)
			controllerID = "unknown"
		} else {
			slog.Info("controller_id from OS hostname (ha.controller_id пуст; для пула с leader_election — задать id в конфиге)",
				"controller_id", controllerID, "source", "hostname")
		}
	} else {
		slog.Info("controller_id из конфига (API Server / registry sync)", "controller_id", controllerID, "source", "ha.controller_id")
	}

	reg := newRegistryEnvironmentsBuilder(
		inMemoryEnvironmentsRepo,
		environmentRepo,
		materialized,
		schemaRepo,
	)

	return &APIServerSyncUseCase{
		config: cfg,
		registry: reg,
		leader: newLeaderAPIServerStream(
			cfg,
			cfg.APIServer.Address,
			controllerID,
			reg,
			schemaRepo,
			effectiveReconciler,
		),
		etcdFollow: newEtcdFollowerWatch(
			cfg,
			etcdClient,
			schemaCache,
			bundleEnvIndex,
			effectiveReconciler,
			reg,
		),
	}
}

// RegisterAndStream keeps a long-lived connection to API Server: register, heartbeat, snapshot stream.
// On any failure it backs off and reconnects without restarting the process.
func (uc *APIServerSyncUseCase) RegisterAndStream(ctx context.Context) error {
	return uc.leader.registerAndStream(ctx)
}

// StartEtcdFollowerWatch rebuilds xDS from controller etcd when the leader (or another writer) changes data.
// Every replica runs this so snapshots stay aligned without each one streaming from API Server.
func (uc *APIServerSyncUseCase) StartEtcdFollowerWatch(ctx context.Context) {
	uc.etcdFollow.start(ctx)
}
