package container

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"merionyx/api-gateway/internal/api-server/config"
	grpchandler "merionyx/api-gateway/internal/api-server/delivery/grpc/handler"
	httphandler "merionyx/api-gateway/internal/api-server/delivery/http/handler"
	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/repository/etcd"
	"merionyx/api-gateway/internal/api-server/usecase"
	"merionyx/api-gateway/internal/shared/election"
	sharedetcd "merionyx/api-gateway/internal/shared/etcd"

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

	ControllerRegistryHandler *grpchandler.ControllerRegistryHandler
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initEtcd()
	container.initLeaderGate()
	container.initRepositories()
	container.initUseCases()
	container.initHandlers()
	container.startBackgroundWork()

	return container, nil
}

func (c *Container) initEtcd() {
	etcdClient, err := sharedetcd.NewEtcdClient(c.Config.Etcd)
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
		slog.Info("API server leader election disabled (noop gate)")
		return
	}

	id := strings.TrimSpace(c.Config.LeaderElection.Identity)
	if id == "" {
		var err error
		id, err = os.Hostname()
		if err != nil || id == "" {
			id = fmt.Sprintf("api-server-%d", time.Now().UnixNano())
		}
	}

	prefix := strings.TrimSpace(c.Config.LeaderElection.KeyPrefix)
	if prefix == "" {
		prefix = "/api-gateway/api-server/election/leader"
	}

	ttl := c.Config.LeaderElection.SessionTTLSeconds
	g := election.NewEtcdGate(c.EtcdClient, prefix, id, ttl)
	c.LeaderGate = g
	go g.Run(context.Background())
	slog.Info("API server leader election started", "prefix", prefix, "identity", id)
}

func (c *Container) initRepositories() {
	c.SnapshotRepository = etcd.NewSnapshotRepository(c.EtcdClient)
	c.ControllerRepository = etcd.NewControllerRepository(c.EtcdClient)

	slog.Info("repositories initialized")
}

func (c *Container) initUseCases() {
	var err error

	c.JWTUseCase, err = usecase.NewJWTUseCase(
		c.Config.JWT.KeysDir,
		c.Config.JWT.Issuer,
	)
	if err != nil {
		slog.Error("failed to initialize JWT use case", "error", err)
		os.Exit(1)
	}

	c.ControllerRegistryUseCase = usecase.NewControllerRegistryUseCase(
		c.ControllerRepository,
		c.SnapshotRepository,
		c.EtcdClient,
	)

	bundleSyncUseCase := usecase.NewBundleSyncUseCase(
		c.SnapshotRepository,
		c.ControllerRepository,
		c.Config.ContractSyncer.Address,
		c.LeaderGate,
	)

	bundleSyncUseCase.SetRegistryUseCase(c.ControllerRegistryUseCase.(*usecase.ControllerRegistryUseCase))

	c.BundleSyncUseCase = bundleSyncUseCase

	slog.Info("use cases initialized")
}

func (c *Container) initHandlers() {
	c.JWTHandler = httphandler.NewJWTHandler(c.JWTUseCase)
	c.ControllerRegistryHandler = grpchandler.NewControllerRegistryHandler(c.ControllerRegistryUseCase)

	slog.Info("handlers initialized")
}

func (c *Container) startBackgroundWork() {
	reg := c.ControllerRegistryUseCase.(*usecase.ControllerRegistryUseCase)
	go reg.StartEtcdWatch(context.Background())
	go c.BundleSyncUseCase.StartBundleWatcher(context.Background())
	slog.Info("API server etcd watch and bundle watcher started")
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
