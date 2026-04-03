package container

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/discovery/kubernetes"
	"merionyx/api-gateway/internal/controller/delivery/grpc/handler"
	"merionyx/api-gateway/internal/controller/delivery/http/router"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	ctrlrepoetcd "merionyx/api-gateway/internal/controller/repository/etcd"
	"merionyx/api-gateway/internal/controller/repository/memory"
	"merionyx/api-gateway/internal/controller/usecase"
	"merionyx/api-gateway/internal/shared/election"
	"merionyx/api-gateway/internal/shared/etcd"

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
	AuthGRPCHandler         *handler.AuthHandler

	Router *router.Router

	XDSSnapshotManager *xdscache.SnapshotManager
	XDSServer          *xdsserver.Server
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initEtcd()
	container.initLeaderGate()
	container.initXDS()

	container.createInMemoryServiceRepository()
	container.initRepositories()

	container.initXDSBuilder()

	container.initUseCases()
	container.initHandlers()
	container.initRouter()
	container.initInMemoryRepositories()
	container.startWatchers()

	return container, nil
}

func (c *Container) initEtcd() {
	etcdClient, err := etcd.NewEtcdClient(c.Config.Etcd)
	if err != nil {
		slog.Error("failed to initialize etcd client", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := etcdClient.Status(ctx, c.Config.Etcd.Endpoints[0]); err != nil {
		slog.Error("failed to connect to etcd", "error", err)
		os.Exit(1)
	}

	c.EtcdClient = etcdClient
	slog.Info("etcd client initialized and connected successfully")
}

func (c *Container) initLeaderGate() {
	if !c.Config.LeaderElection.Enabled {
		c.LeaderGate = election.NoopGate{}
		slog.Info("gateway controller leader election disabled (noop gate)")
		return
	}

	id := strings.TrimSpace(c.Config.LeaderElection.Identity)
	if id == "" {
		var err error
		id, err = os.Hostname()
		if err != nil || id == "" {
			id = fmt.Sprintf("controller-%d", time.Now().UnixNano())
		}
	}

	prefix := strings.TrimSpace(c.Config.LeaderElection.KeyPrefix)
	if prefix == "" {
		prefix = "/api-gateway/controller/election/leader"
	}

	g := election.NewEtcdGate(c.EtcdClient, prefix, id, c.Config.LeaderElection.SessionTTLSeconds)
	c.LeaderGate = g
	go g.Run(context.Background())
	slog.Info("gateway controller leader election started", "prefix", prefix, "identity", id)
}

func (c *Container) createInMemoryServiceRepository() {
	c.InMemoryServiceRepository = memory.NewServiceRepository()
	c.InMemoryEnvironmentsRepository = memory.NewEnvironmentsRepository()
}

func (c *Container) initInMemoryRepositories() {
	c.InMemoryEnvironmentsRepository.SetDependencies(c.XDSSnapshotManager, c.XDSBuilder, c.SchemaRepository)

	if err := c.InMemoryServiceRepository.Initialize(c.Config); err != nil {
		slog.Error("failed to initialize service repository", "error", err)
		os.Exit(1)
	}

	if err := c.InMemoryEnvironmentsRepository.Initialize(c.Config); err != nil {
		slog.Error("failed to initialize environments repository", "error", err)
		os.Exit(1)
	}

	slog.Info("in-memory repositories initialized")
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

func (c *Container) initRouter() {
	c.Router = router.NewRouter(
	// c.TenantUseCase,
	// c.EnvironmentUseCase,
	// c.ListenerUseCase,
	)

	slog.Info("router initialized")
}

func (c *Container) initXDS() {
	c.XDSSnapshotManager = xdscache.NewSnapshotManager()
	xdsPort, err := strconv.Atoi(c.Config.Server.XDSPort)
	if err != nil {
		slog.Error("failed to convert xDS port to int", "error", err)
		os.Exit(1)
	}
	c.XDSServer = xdsserver.NewXDSServer(c.XDSSnapshotManager.GetCache(), xdsPort)

	slog.Info("xDS server initialized")
}

func (c *Container) startWatchers() {
	go c.APIServerSyncUseCase.StartEtcdFollowerWatch(context.Background())

	go func() {
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		var syncCancel context.CancelFunc
		for range tick.C {
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
	}()
}

// StartKubernetesDiscovery runs a background sync from gateway.merionyx.io CRs (cluster-wide).
func (c *Container) StartKubernetesDiscovery(ctx context.Context) {
	if c.Config.KubernetesDiscovery == nil || !c.Config.KubernetesDiscovery.Enabled {
		return
	}
	envRepo, ok1 := c.InMemoryEnvironmentsRepository.(*memory.EnvironmentsRepository)
	svcRepo, ok2 := c.InMemoryServiceRepository.(*memory.ServiceRepository)
	if !ok1 || !ok2 {
		slog.Error("kubernetes discovery: unexpected in-memory repository implementation")
		return
	}
	runner, err := kubernetes.NewRunner(c.Config.KubernetesDiscovery, envRepo, svcRepo)
	if err != nil {
		slog.Error("kubernetes discovery: init client", "error", err)
		return
	}
	go runner.Run(ctx, 15*time.Second)
	slog.Info("kubernetes discovery started")
}

// Close closes all resources in the container
func (c *Container) Close() {
	if c.EtcdClient != nil {
		if err := c.EtcdClient.Close(); err != nil {
			slog.Error("failed to close etcd client", "error", err)
		}
		slog.Info("etcd client closed")
	}
}
