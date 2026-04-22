package usecase

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/controller/repository/cache"
	ctrlrepoetcd "github.com/merionyx/api-gateway/internal/controller/repository/etcd"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/shared/grpcutil"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// APIServerSyncUseCase keeps the Gateway Controller in sync with API Server (leader stream) and
// reconciles xDS from controller-local etcd on every replica (follower watch).
type APIServerSyncUseCase struct {
	config                   *config.Config
	schemaRepo               interfaces.SchemaRepository
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository
	environmentRepo          interfaces.EnvironmentRepository
	xdsSnapshotManager       *xdscache.SnapshotManager
	apiServerAddress         string
	controllerID             string
	xdsBuilder               interfaces.XDSBuilder
	etcdClient               *clientv3.Client
	bundleEnvIndex           *bundleenv.Index
	schemaCache              *cache.SchemaCache
	materialized             *ctrlrepoetcd.MaterializedStore
}

func NewAPIServerSyncUseCase(
	cfg *config.Config,
	schemaRepo interfaces.SchemaRepository,
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository,
	environmentRepo interfaces.EnvironmentRepository,
	xdsSnapshotManager *xdscache.SnapshotManager,
	xdsBuilder interfaces.XDSBuilder,
	etcdClient *clientv3.Client,
	bundleEnvIndex *bundleenv.Index,
	schemaCache *cache.SchemaCache,
	materialized *ctrlrepoetcd.MaterializedStore,
) *APIServerSyncUseCase {
	controllerID := strings.TrimSpace(cfg.HA.ControllerID)
	if controllerID == "" {
		var err error
		controllerID, err = os.Hostname()
		if err != nil {
			slog.Error("Failed to get hostname", "error", err)
			controllerID = "unknown"
		}
	}

	return &APIServerSyncUseCase{
		config:                   cfg,
		schemaRepo:               schemaRepo,
		inMemoryEnvironmentsRepo: inMemoryEnvironmentsRepo,
		environmentRepo:          environmentRepo,
		xdsSnapshotManager:       xdsSnapshotManager,
		apiServerAddress:         cfg.APIServer.Address,
		xdsBuilder:               xdsBuilder,
		controllerID:             controllerID,
		etcdClient:               etcdClient,
		bundleEnvIndex:           bundleEnvIndex,
		schemaCache:              schemaCache,
		materialized:             materialized,
	}
}

// RegisterAndStream keeps a long-lived connection to API Server: register, heartbeat, snapshot stream.
// On any failure it backs off and reconnects without restarting the process.
func (uc *APIServerSyncUseCase) RegisterAndStream(ctx context.Context) error {
	const (
		initialBackoff = time.Second
		maxBackoff     = 60 * time.Second
	)
	backoff := time.Duration(0)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if backoff > 0 {
			slog.Warn("Reconnecting to API Server after backoff", "address", uc.apiServerAddress, "backoff", backoff)
			if err := grpcutil.SleepOrDone(ctx, backoff); err != nil {
				return err
			}
		}

		slog.Info("Connecting to API Server", "address", uc.apiServerAddress)
		sessErr := uc.runAPIServerSession(ctx)
		if err := ctx.Err(); err != nil {
			ctrlmetrics.RecordAPIServerSessionEnd(uc.config.MetricsHTTP.Enabled, ctrlmetrics.SessionReasonCanceled)
			return err
		}
		if sessErr == nil {
			return nil
		}
		if errors.Is(sessErr, context.Canceled) {
			ctrlmetrics.RecordAPIServerSessionEnd(uc.config.MetricsHTTP.Enabled, ctrlmetrics.SessionReasonCanceled)
			return sessErr
		}

		ctrlmetrics.RecordAPIServerSessionEnd(uc.config.MetricsHTTP.Enabled, ctrlmetrics.SessionReasonError)
		slog.Warn("API Server sync session ended", "error", sessErr)
		backoff = grpcutil.NextReconnectBackoff(backoff, initialBackoff, maxBackoff)
	}
}
