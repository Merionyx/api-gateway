package container

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/delivery/grpc/handler"
	"merionyx/api-gateway/internal/controller/delivery/http/router"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	etcd_repository "merionyx/api-gateway/internal/controller/repository/etcd"
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
		log.Fatalf("Failed to initialize etcd client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := etcdClient.Status(ctx, c.Config.Etcd.Endpoints[0]); err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}

	c.EtcdClient = etcdClient
	log.Println("etcd client initialized and connected successfully")
}

func (c *Container) initLeaderGate() {
	if !c.Config.LeaderElection.Enabled {
		c.LeaderGate = election.NoopGate{}
		log.Println("Gateway Controller leader election disabled (noop gate)")
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
	log.Printf("Gateway Controller leader election started (prefix=%s identity=%s)", prefix, id)
}

func (c *Container) createInMemoryServiceRepository() {
	c.InMemoryServiceRepository = memory.NewServiceRepository()
	c.InMemoryEnvironmentsRepository = memory.NewEnvironmentsRepository()
}

func (c *Container) initInMemoryRepositories() {
	c.InMemoryEnvironmentsRepository.SetDependencies(c.XDSSnapshotManager, c.XDSBuilder)

	if err := c.InMemoryServiceRepository.Initialize(c.Config); err != nil {
		log.Fatalf("Failed to initialize service repository: %v", err)
	}

	if err := c.InMemoryEnvironmentsRepository.Initialize(c.Config); err != nil {
		log.Fatalf("Failed to initialize environments repository: %v", err)
	}

	log.Println("In-memory repositories initialized")
}

func (c *Container) initXDSBuilder() {
	c.XDSBuilder = builder.NewXDSBuilder(c.InMemoryServiceRepository)

	log.Println("xDS builder initialized")
}

func (c *Container) initRepositories() {
	c.SchemaRepository = etcd_repository.NewSchemaRepository(c.EtcdClient)
	c.EnvironmentRepository = etcd_repository.NewEnvironmentRepository(c.EtcdClient)

	log.Println("etcd repositories initialized")
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
		c.EnvironmentsUseCase,
		c.XDSSnapshotManager,
		c.XDSBuilder,
		c.EtcdClient,
	)

	log.Println("SnapshotsUseCase:", c.SnapshotsUseCase)

	log.Println("Use cases initialized")
}

func (c *Container) initHandlers() {
	c.SnapshotGRPCHandler = handler.NewSnapshotHandler(c.SnapshotsUseCase)
	c.EnvironmentsGRPCHandler = handler.NewEnvironmentsHandler(c.EnvironmentsUseCase, c.LeaderGate)
	c.SchemasGRPCHandler = handler.NewSchemasHandler(c.SchemasUseCase)
	c.AuthGRPCHandler = handler.NewAuthHandler(c.EnvironmentRepository, c.InMemoryEnvironmentsRepository, c.SchemaRepository, c.EtcdClient)

	log.Println("Handlers initialized")
}

func (c *Container) initRouter() {
	c.Router = router.NewRouter(
	// c.TenantUseCase,
	// c.EnvironmentUseCase,
	// c.ListenerUseCase,
	)

	log.Println("Router initialized")
}

func (c *Container) initXDS() {
	c.XDSSnapshotManager = xdscache.NewSnapshotManager()
	xdsPort, err := strconv.Atoi(c.Config.Server.XDSPort)
	if err != nil {
		log.Fatalf("Failed to convert xDS port to int: %v", err)
	}
	c.XDSServer = xdsserver.NewXDSServer(c.XDSSnapshotManager.GetCache(), xdsPort)

	log.Println("xDS server initialized")
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
							log.Printf("API Server sync ended: %v", err)
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

// Close closes all resources in the container
func (c *Container) Close() {
	if c.EtcdClient != nil {
		if err := c.EtcdClient.Close(); err != nil {
			log.Printf("Failed to close etcd client: %v", err)
		}
		log.Println("etcd client closed")
	}
}
