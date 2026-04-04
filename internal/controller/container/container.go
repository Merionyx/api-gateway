package container

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/discovery/kubernetes"
	"merionyx/api-gateway/internal/controller/delivery/grpc/handler"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	ctrlrepoetcd "merionyx/api-gateway/internal/controller/repository/etcd"
	"merionyx/api-gateway/internal/controller/repository/memory"
	"merionyx/api-gateway/internal/controller/usecase"
	"merionyx/api-gateway/internal/shared/bootstrap"
	"merionyx/api-gateway/internal/shared/election"
	"merionyx/api-gateway/internal/shared/grpcobs"

	"merionyx/api-gateway/internal/controller/xds/builder"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	xdsserver "merionyx/api-gateway/internal/controller/xds/server"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Container DI for all dependencies
type Container struct {
	Config *config.Config

	EtcdClient *clientv3.Client
	LeaderGate election.LeaderGate

	SchemaRepository      interfaces.SchemaRepository
	EnvironmentRepository interfaces.EnvironmentRepository

	InMemoryServiceRepository      interfaces.InMemoryServiceRepository
	InMemoryEnvironmentsRepository interfaces.InMemoryEnvironmentsRepository

	XDSBuilder interfaces.XDSBuilder

	SnapshotsUseCase     interfaces.SnapshotsUseCase
	EnvironmentsUseCase  interfaces.EnvironmentsUseCase
	SchemasUseCase       interfaces.SchemasUseCase
	APIServerSyncUseCase *usecase.APIServerSyncUseCase

	SnapshotGRPCHandler     *handler.SnapshotHandler
	EnvironmentsGRPCHandler *handler.EnvironmentsHandler
	SchemasGRPCHandler      *handler.SchemasHandler
	AuthGRPCHandler *handler.AuthHandler

	XDSSnapshotManager *xdscache.SnapshotManager
	XDSServer          *xdsserver.Server
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
	if err := c.initXDS(); err != nil {
		return nil, err
	}

	c.createInMemoryServiceRepository()
	c.initRepositories()

	c.initXDSBuilder()

	c.initUseCases()
	c.initHandlers()
	if err := c.initInMemoryRepositories(); err != nil {
		return nil, err
	}
	c.startWatchers()

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
		DefaultKeyPrefix:  "/api-gateway/controller/election/leader",
		FallbackIDPrefix:  "controller",
		Service:           "gateway-controller",
	})
}

func (c *Container) createInMemoryServiceRepository() {
	c.InMemoryServiceRepository = memory.NewServiceRepository()
	c.InMemoryEnvironmentsRepository = memory.NewEnvironmentsRepository()
}

func (c *Container) initInMemoryRepositories() error {
	c.InMemoryEnvironmentsRepository.SetDependencies(c.XDSSnapshotManager, c.XDSBuilder, c.SchemaRepository)

	if err := c.InMemoryServiceRepository.Initialize(c.Config); err != nil {
		return fmt.Errorf("initialize service repository: %w", err)
	}

	if err := c.InMemoryEnvironmentsRepository.Initialize(c.Config); err != nil {
		return fmt.Errorf("initialize environments repository: %w", err)
	}

	slog.Info("in-memory repositories initialized")
	return nil
}

func (c *Container) initXDSBuilder() {
	c.XDSBuilder = builder.NewXDSBuilder(c.InMemoryServiceRepository)

	slog.Info("xDS builder initialized")
}

func (c *Container) initRepositories() {
	c.SchemaRepository = ctrlrepoetcd.NewSchemaRepository(c.EtcdClient)
	c.EnvironmentRepository = ctrlrepoetcd.NewEnvironmentRepository(c.EtcdClient)

	slog.Info("etcd repositories initialized")
}

func (c *Container) initUseCases() {
	c.SnapshotsUseCase = usecase.NewSnapshotsUseCase()
	c.EnvironmentsUseCase = usecase.NewEnvironmentsUseCase()
	c.SchemasUseCase = usecase.NewSchemasUseCase()

	// Two-phase wiring: use cases reference each other; order matches dependency direction (schemas → env → snapshots).
	c.EnvironmentsUseCase.SetDependencies(c.EnvironmentRepository, c.SchemasUseCase, c.XDSSnapshotManager, c.XDSBuilder)
	c.SnapshotsUseCase.SetDependencies(c.EnvironmentsUseCase, c.XDSSnapshotManager, c.XDSBuilder)
	c.SchemasUseCase.SetDependencies(c.SchemaRepository, c.EnvironmentRepository)

	c.APIServerSyncUseCase = usecase.NewAPIServerSyncUseCase(
		c.Config,
		c.SchemaRepository,
		c.InMemoryEnvironmentsRepository,
		c.EnvironmentRepository,
		c.XDSSnapshotManager,
		c.XDSBuilder,
		c.EtcdClient,
	)

	slog.Info("use cases initialized")
}

func (c *Container) initHandlers() {
	c.SnapshotGRPCHandler = handler.NewSnapshotHandler(c.SnapshotsUseCase)
	c.EnvironmentsGRPCHandler = handler.NewEnvironmentsHandler(c.EnvironmentsUseCase, c.LeaderGate)
	c.SchemasGRPCHandler = handler.NewSchemasHandler(c.SchemasUseCase)
	c.AuthGRPCHandler = handler.NewAuthHandler(c.EnvironmentRepository, c.InMemoryEnvironmentsRepository, c.SchemaRepository, c.EtcdClient)

	slog.Info("handlers initialized")
}

func (c *Container) initXDS() error {
	c.XDSSnapshotManager = xdscache.NewSnapshotManager(c.Config.MetricsHTTP.Enabled)
	xdsOpts, err := grpcobs.ServerOptions(&c.Config.GRPCXDS.TLS, c.Config.GRPCXDS.Observability, c.Config.MetricsHTTP.Enabled)
	if err != nil {
		return fmt.Errorf("xDS gRPC options: %w", err)
	}
	c.XDSServer = xdsserver.NewXDSServer(
		c.XDSSnapshotManager.GetCache(),
		c.Config.GRPCXDS.Observability.ReflectionEnabled,
		xdsOpts...,
	)

	slog.Info("xDS server initialized")
	return nil
}

func (c *Container) startWatchers() {
	go c.APIServerSyncUseCase.StartEtcdFollowerWatch(context.Background())

	go func() {
		var syncCancel context.CancelFunc
		defer func() {
			if syncCancel != nil {
				syncCancel()
			}
		}()

		reconcile := func() {
			if c.LeaderGate.IsLeader() {
				if syncCancel == nil {
					var sctx context.Context
					sctx, syncCancel = context.WithCancel(context.Background())
					go func() {
						if err := c.APIServerSyncUseCase.RegisterAndStream(sctx); err != nil {
							slog.Warn("API server sync ended", "error", err)
						}
					}()
				}
			} else {
				if syncCancel != nil {
					syncCancel()
					syncCancel = nil
				}
			}
		}

		reconcile()
		ch := c.LeaderGate.LeaderChanged()
		if ch == nil {
			return
		}
		for range ch {
			reconcile()
		}
	}()
}

// StartKubernetesDiscovery runs a background sync from gateway.merionyx.io CRs (cluster-wide).
func (c *Container) StartKubernetesDiscovery(ctx context.Context) {
	if c.Config.KubernetesDiscovery == nil || !c.Config.KubernetesDiscovery.Enabled {
		return
	}
	runner, err := kubernetes.NewRunner(c.Config.KubernetesDiscovery, c.InMemoryEnvironmentsRepository, c.InMemoryServiceRepository)
	if err != nil {
		slog.Error("kubernetes discovery: init client", "error", err)
		return
	}
	go runner.Run(ctx, 15*time.Second)
	slog.Info("kubernetes discovery started")
}

// Close closes all resources in the container
func (c *Container) Close() {
	bootstrap.CloseEtcdClient(c.EtcdClient)
}
