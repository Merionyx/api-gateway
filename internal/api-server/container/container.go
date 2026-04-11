package container

import (
	"context"
	"fmt"
	"log/slog"

	contractsyncergrpc "github.com/merionyx/api-gateway/internal/api-server/adapter/contractsyncer/grpc"
	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	grpchandler "github.com/merionyx/api-gateway/internal/api-server/delivery/grpc/handler"
	httphandler "github.com/merionyx/api-gateway/internal/api-server/delivery/http/handler"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/registry"
	"github.com/merionyx/api-gateway/internal/shared/bootstrap"
	"github.com/merionyx/api-gateway/internal/shared/election"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Container DI for all dependencies
type Container struct {
	Config *config.Config

	EtcdClient *clientv3.Client
	LeaderGate election.LeaderGate

	SnapshotRepository   interfaces.SnapshotRepository
	ControllerRepository interfaces.ControllerRepository

	// ContractSyncerGRPC is the gRPC adapter for Contract Syncer (sync, export, ping).
	ContractSyncerGRPC *contractsyncergrpc.Client

	JWTUseCase                *auth.JWTUseCase
	ControllerRegistryUseCase interfaces.ControllerRegistryUseCase
	BundleSyncUseCase         interfaces.BundleSyncUseCase

	JWTHandler *httphandler.JWTHandler

	ContractsExportHandler *httphandler.ContractsExportHandler

	RegistryHandler *httphandler.RegistryHandler

	BundleReadUseCase      *bundle.BundleReadUseCase
	ControllerReadUseCase  *registry.ControllerReadUseCase
	TenantReadUseCase      *registry.TenantReadUseCase
	BundleHTTPSyncUseCase  *bundle.BundleHTTPSyncUseCase
	StatusReadUseCase      *registry.StatusReadUseCase

	ControllerRegistryHandler *grpchandler.ControllerRegistryHandler
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	c := &Container{
		Config: cfg,
	}

	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()

	if err := c.initEtcd(); err != nil {
		return nil, err
	}
	c.initLeaderGate()
	c.initRepositories()
	if err := c.initUseCases(); err != nil {
		return nil, err
	}
	c.initHandlers()

	ok = true
	return c, nil
}

func (c *Container) initEtcd() error {
	client, err := bootstrap.ConnectEtcd(context.Background(), c.Config.Etcd, bootstrap.DefaultEtcdStatusTimeout)
	if err != nil {
		return err
	}
	c.EtcdClient = client
	return nil
}

func (c *Container) initLeaderGate() {
	le := c.Config.LeaderElection
	c.LeaderGate = bootstrap.StartLeaderElection(context.Background(), c.EtcdClient, bootstrap.LeaderElectionSettings{
		Enabled:           le.Enabled,
		Identity:          le.Identity,
		KeyPrefix:         le.KeyPrefix,
		SessionTTLSeconds: le.SessionTTLSeconds,
		DefaultKeyPrefix:  "/api-gateway/api-server/election/leader",
		FallbackIDPrefix:  "api-server",
		Service:           "api-server",
	})
}

func (c *Container) initRepositories() {
	c.SnapshotRepository = etcd.NewSnapshotRepository(c.EtcdClient)
	c.ControllerRepository = etcd.NewControllerRepository(c.EtcdClient)

	slog.Info("repositories initialized")
}

func (c *Container) initUseCases() error {
	jwtUC, err := auth.NewJWTUseCase(
		c.Config.JWT.KeysDir,
		c.Config.JWT.Issuer,
	)
	if err != nil {
		return fmt.Errorf("initialize JWT use case: %w", err)
	}
	c.JWTUseCase = jwtUC

	c.ControllerRegistryUseCase = registry.NewControllerRegistryUseCase(
		c.ControllerRepository,
		c.SnapshotRepository,
		c.EtcdClient,
	)

	c.ContractSyncerGRPC = contractsyncergrpc.NewClient(c.Config.ContractSyncer.Address, c.Config.GRPCContractSyncerClient)

	c.BundleSyncUseCase = bundle.NewBundleSyncUseCase(
		c.SnapshotRepository,
		c.ControllerRepository,
		c.ContractSyncerGRPC,
		c.LeaderGate,
		c.Config.MetricsHTTP.Enabled,
	)

	c.BundleReadUseCase = bundle.NewBundleReadUseCase(c.SnapshotRepository)
	c.ControllerReadUseCase = registry.NewControllerReadUseCase(c.ControllerRepository)
	c.TenantReadUseCase = registry.NewTenantReadUseCase(c.ControllerRepository)
	c.BundleHTTPSyncUseCase = bundle.NewBundleHTTPSyncUseCase(c.SnapshotRepository, c.BundleSyncUseCase)
	c.StatusReadUseCase = registry.NewStatusReadUseCase(
		c.EtcdClient,
		c.ContractSyncerGRPC,
	)

	slog.Info("use cases initialized")
	return nil
}

func (c *Container) initHandlers() {
	c.JWTHandler = httphandler.NewJWTHandler(c.JWTUseCase, c.Config.MetricsHTTP.Enabled)
	exportUC := bundle.NewContractExportUseCase(c.ContractSyncerGRPC)
	c.ContractsExportHandler = httphandler.NewContractsExportHandler(exportUC)
	c.RegistryHandler = httphandler.NewRegistryHandler(
		c.BundleReadUseCase,
		c.ControllerReadUseCase,
		c.TenantReadUseCase,
		c.BundleHTTPSyncUseCase,
		c.StatusReadUseCase,
	)
	c.ControllerRegistryHandler = grpchandler.NewControllerRegistryHandler(c.ControllerRegistryUseCase)

	slog.Info("handlers initialized")
}

// StartBackgroundWork runs etcd watch and bundle sync until ctx is cancelled.
func (c *Container) StartBackgroundWork(ctx context.Context) {
	go c.ControllerRegistryUseCase.StartEtcdWatch(ctx)
	go c.BundleSyncUseCase.StartBundleWatcher(ctx)
	slog.Info("API server etcd watch and bundle watcher started")
}

// Close closes all resources in the container
func (c *Container) Close() {
	bootstrap.CloseEtcdClient(c.EtcdClient)
}
