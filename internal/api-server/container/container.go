package container

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	grpchandler "github.com/merionyx/api-gateway/internal/api-server/delivery/grpc/handler"
	httphandler "github.com/merionyx/api-gateway/internal/api-server/delivery/http/handler"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/repository/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/usecase"
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

	JWTUseCase                *usecase.JWTUseCase
	ControllerRegistryUseCase interfaces.ControllerRegistryUseCase
	BundleSyncUseCase         interfaces.BundleSyncUseCase

	JWTHandler *httphandler.JWTHandler

	ContractsExportHandler *httphandler.ContractsExportHandler

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
	jwtUC, err := usecase.NewJWTUseCase(
		c.Config.JWT.KeysDir,
		c.Config.JWT.Issuer,
	)
	if err != nil {
		return fmt.Errorf("initialize JWT use case: %w", err)
	}
	c.JWTUseCase = jwtUC

	c.ControllerRegistryUseCase = usecase.NewControllerRegistryUseCase(
		c.ControllerRepository,
		c.SnapshotRepository,
		c.EtcdClient,
	)

	c.BundleSyncUseCase = usecase.NewBundleSyncUseCase(
		c.SnapshotRepository,
		c.ControllerRepository,
		c.Config.ContractSyncer.Address,
		c.Config.GRPCContractSyncerClient,
		c.LeaderGate,
		c.Config.MetricsHTTP.Enabled,
	)

	slog.Info("use cases initialized")
	return nil
}

func (c *Container) initHandlers() {
	c.JWTHandler = httphandler.NewJWTHandler(c.JWTUseCase, c.Config.MetricsHTTP.Enabled)
	exportUC := usecase.NewContractExportUseCase(c.Config.ContractSyncer.Address, c.Config.GRPCContractSyncerClient)
	c.ContractsExportHandler = httphandler.NewContractsExportHandler(exportUC)
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
