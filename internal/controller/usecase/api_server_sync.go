package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/controller/repository/memory"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	xdssnapshot "merionyx/api-gateway/internal/controller/xds/snapshot"
	"merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	"merionyx/api-gateway/internal/shared/grpcutil"
	pb "merionyx/api-gateway/pkg/api/controller_registry/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const controllerEtcdWatchPrefix = "/api-gateway/controller/"

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
}

func NewAPIServerSyncUseCase(
	cfg *config.Config,
	schemaRepo interfaces.SchemaRepository,
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository,
	environmentRepo interfaces.EnvironmentRepository,
	xdsSnapshotManager *xdscache.SnapshotManager,
	xdsBuilder interfaces.XDSBuilder,
	etcdClient *clientv3.Client,
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
	}
}

func (uc *APIServerSyncUseCase) grpcDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
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
			return err
		}
		if sessErr == nil {
			return nil
		}
		if errors.Is(sessErr, context.Canceled) {
			return sessErr
		}

		slog.Warn("API Server sync session ended", "error", sessErr)
		backoff = grpcutil.NextReconnectBackoff(backoff, initialBackoff, maxBackoff)
	}
}

// runAPIServerSession uses one connection: register, heartbeat goroutine, block on snapshot stream.
func (uc *APIServerSyncUseCase) runAPIServerSession(parentCtx context.Context) error {
	conn, err := grpc.NewClient(uc.apiServerAddress, uc.grpcDialOptions()...)
	if err != nil {
		return fmt.Errorf("dial API Server: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			slog.Debug("API Server conn close", "error", cerr)
		}
	}()

	client := pb.NewControllerRegistryServiceClient(conn)
	if err := uc.registerController(parentCtx, client); err != nil {
		return fmt.Errorf("register controller: %w", err)
	}

	sessionCtx, cancelSession := context.WithCancel(parentCtx)
	defer cancelSession()

	go uc.startHeartbeat(sessionCtx, client)

	streamErr := uc.streamSnapshots(sessionCtx, client)
	cancelSession()
	if err := parentCtx.Err(); err != nil {
		return err
	}
	return streamErr
}

func (uc *APIServerSyncUseCase) registerController(ctx context.Context, client pb.ControllerRegistryServiceClient) error {
	environments, err := uc.buildEnvironmentsForAPIServer(ctx)
	if err != nil {
		return err
	}

	resp, err := client.RegisterController(ctx, &pb.RegisterControllerRequest{
		ControllerId: uc.controllerID,
		Tenant:       uc.config.Tenant,
		Environments: environments,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Error)
	}

	slog.Info("Successfully registered with API Server", "controller_id", uc.controllerID, "environments_count", len(environments))
	return nil
}

func (uc *APIServerSyncUseCase) startHeartbeat(ctx context.Context, client pb.ControllerRegistryServiceClient) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			environments, err := uc.buildEnvironmentsForAPIServer(ctx)
			if err != nil {
				slog.Error("Failed to build environments for heartbeat", "error", err)
				continue
			}

			_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{
				ControllerId: uc.controllerID,
				Environments: environments,
			})
			if err != nil {
				slog.Error("Failed to send heartbeat", "error", err)
			}
		}
	}
}

func (uc *APIServerSyncUseCase) streamSnapshots(ctx context.Context, client pb.ControllerRegistryServiceClient) error {
	stream, err := client.StreamSnapshots(ctx, &pb.StreamSnapshotsRequest{
		ControllerId: uc.controllerID,
	})
	if err != nil {
		return fmt.Errorf("open StreamSnapshots: %w", err)
	}

	slog.Info("Started receiving snapshot stream")

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("snapshot stream closed by server: %w", err)
			}
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.Canceled, codes.DeadlineExceeded:
					if ctx.Err() != nil {
						return ctx.Err()
					}
				}
			}
			return fmt.Errorf("snapshot stream recv: %w", err)
		}

		slog.Info("Received snapshots", "environment", resp.Environment, "bundle_key", resp.BundleKey, "count", len(resp.Snapshots))

		if len(resp.Snapshots) == 0 {
			slog.Debug("Skipping empty snapshot batch to avoid clearing xDS", "environment", resp.Environment, "bundle_key", resp.BundleKey)
			continue
		}

		var snapshots []sharedgit.ContractSnapshot
		for _, pbSnapshot := range resp.Snapshots {
			var apps []sharedgit.App
			if acc := pbSnapshot.GetAccess(); acc != nil {
				for _, pbApp := range acc.GetApps() {
					apps = append(apps, sharedgit.App{
						AppID:        pbApp.GetAppId(),
						Environments: pbApp.GetEnvironments(),
					})
				}
			}
			upstreamName := ""
			if u := pbSnapshot.GetUpstream(); u != nil {
				upstreamName = u.GetName()
			}
			secure := false
			if acc := pbSnapshot.GetAccess(); acc != nil {
				secure = acc.GetSecure()
			}
			snapshots = append(snapshots, sharedgit.ContractSnapshot{
				Name:                  pbSnapshot.GetName(),
				Prefix:                pbSnapshot.GetPrefix(),
				Upstream:              sharedgit.ContractUpstream{Name: upstreamName},
				AllowUndefinedMethods: pbSnapshot.GetAllowUndefinedMethods(),
				Access: sharedgit.Access{
					Secure: secure,
					Apps:   apps,
				},
			})
		}

		if err := uc.saveSnapshotsToEtcd(ctx, resp.BundleKey, snapshots); err != nil {
			slog.Error("Failed to save snapshots to etcd", "error", err)
			continue
		}

		if err := uc.updateXDSSnapshot(ctx, resp.Environment); err != nil {
			slog.Error("Failed to update xDS snapshot", "error", err)
		}
	}
}

func (uc *APIServerSyncUseCase) saveSnapshotsToEtcd(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	repository, ref, bundlePath, err := bundlekey.Parse(bundleKey)
	if err != nil {
		return err
	}

	for _, s := range snapshots {
		cs := sharedToControllerSnapshot(s)
		slog.Info("Saving snapshot to etcd", "repository", repository, "ref", ref, "path", bundlePath, "contract", s.Name)
		if err := uc.schemaRepo.SaveContractSnapshot(ctx, repository, ref, bundlePath, s.Name, cs); err != nil {
			return fmt.Errorf("save snapshot %s: %w", s.Name, err)
		}
	}
	return nil
}

func (uc *APIServerSyncUseCase) updateXDSSnapshot(ctx context.Context, environment string) error {
	slog.Info("Updating xDS snapshot", "environment", environment)

	env, err := uc.environmentForXDS(ctx, environment)
	if err != nil {
		return err
	}

	xdsSnap, err := xdssnapshot.BuildEnvoySnapshot(uc.xdsBuilder, env)
	if err != nil {
		return fmt.Errorf("build envoy snapshot: %w", err)
	}
	nodeID := fmt.Sprintf("envoy-%s", environment)
	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnap); err != nil {
		return fmt.Errorf("failed to push xDS snapshot: %w", err)
	}
	return nil
}

func (uc *APIServerSyncUseCase) environmentForXDS(ctx context.Context, name string) (*models.Environment, error) {
	skel, err := uc.effectiveEnvironmentSkeleton(ctx, name)
	if err != nil {
		return nil, err
	}
	return uc.environmentWithSnapshotsFromSchema(ctx, skel), nil
}

// effectiveEnvironmentSkeleton merges static+Kubernetes (in-memory) with controller etcd CRUD:
// union of bundles and services. If only one side exists, that side is used.
func (uc *APIServerSyncUseCase) effectiveEnvironmentSkeleton(ctx context.Context, name string) (*models.Environment, error) {
	var mem *models.Environment
	if m, err := uc.inMemoryEnvironmentsRepo.GetEnvironment(ctx, name); err == nil {
		mem = m
	}

	var etcdEnv *models.Environment
	if uc.environmentRepo != nil {
		if e, err := uc.environmentRepo.GetEnvironment(ctx, name); err == nil {
			etcdEnv = e
		}
	}

	if mem == nil && etcdEnv == nil {
		return nil, fmt.Errorf("environment %s not found", name)
	}
	if mem == nil {
		return skeletonFromEtcdOnly(etcdEnv), nil
	}
	if etcdEnv == nil {
		return skeletonFromMemory(mem), nil
	}

	uB := memory.UnionStaticContractBundles(staticBundles(mem), staticBundles(etcdEnv))
	uS := memory.UnionStaticServices(staticServices(mem), staticServices(etcdEnv))
	return &models.Environment{
		Name:      mem.Name,
		Type:      mem.Type,
		Bundles:   &models.EnvironmentBundleConfig{Static: uB},
		Services:  &models.EnvironmentServiceConfig{Static: uS},
		Snapshots: nil,
	}, nil
}

func skeletonFromMemory(mem *models.Environment) *models.Environment {
	return &models.Environment{
		Name:      mem.Name,
		Type:      mem.Type,
		Bundles:   cloneBundlesConfig(mem.Bundles),
		Services:  cloneServicesConfig(mem.Services),
		Snapshots: nil,
	}
}

func skeletonFromEtcdOnly(etcdEnv *models.Environment) *models.Environment {
	return &models.Environment{
		Name:      etcdEnv.Name,
		Type:      etcdEnv.Type,
		Bundles:   cloneBundlesConfig(etcdEnv.Bundles),
		Services:  cloneServicesConfig(etcdEnv.Services),
		Snapshots: nil,
	}
}

func staticBundles(e *models.Environment) []models.StaticContractBundleConfig {
	if e == nil || e.Bundles == nil {
		return nil
	}
	return e.Bundles.Static
}

func staticServices(e *models.Environment) []models.StaticServiceConfig {
	if e == nil || e.Services == nil {
		return nil
	}
	return e.Services.Static
}

func cloneBundlesConfig(b *models.EnvironmentBundleConfig) *models.EnvironmentBundleConfig {
	if b == nil {
		return &models.EnvironmentBundleConfig{Static: nil}
	}
	cp := make([]models.StaticContractBundleConfig, len(b.Static))
	copy(cp, b.Static)
	return &models.EnvironmentBundleConfig{Static: cp}
}

func cloneServicesConfig(s *models.EnvironmentServiceConfig) *models.EnvironmentServiceConfig {
	if s == nil {
		return &models.EnvironmentServiceConfig{Static: nil}
	}
	cp := make([]models.StaticServiceConfig, len(s.Static))
	copy(cp, s.Static)
	return &models.EnvironmentServiceConfig{Static: cp}
}

// buildEnvironmentsForAPIServer returns the full declared environment set for Register/Heartbeat
// (union of names from in-memory and etcd CRUD), with merged bundles per name.
func (uc *APIServerSyncUseCase) buildEnvironmentsForAPIServer(ctx context.Context) ([]*pb.EnvironmentInfo, error) {
	names := uc.collectEnvironmentNames(ctx)
	sort.Strings(names)
	out := make([]*pb.EnvironmentInfo, 0, len(names))
	for _, n := range names {
		skel, err := uc.effectiveEnvironmentSkeleton(ctx, n)
		if err != nil {
			slog.Warn("buildEnvironmentsForAPIServer: skip environment", "name", n, "error", err)
			continue
		}
		var bundles []*pb.BundleInfo
		if skel.Bundles != nil {
			for _, b := range skel.Bundles.Static {
				bundles = append(bundles, &pb.BundleInfo{
					Name:       b.Name,
					Repository: b.Repository,
					Ref:        b.Ref,
					Path:       b.Path,
				})
			}
		}
		out = append(out, &pb.EnvironmentInfo{Name: skel.Name, Bundles: bundles})
	}
	return out, nil
}

func (uc *APIServerSyncUseCase) collectEnvironmentNames(ctx context.Context) []string {
	names := make(map[string]struct{})
	if m, err := uc.inMemoryEnvironmentsRepo.ListEnvironments(ctx); err != nil {
		slog.Warn("collectEnvironmentNames: in-memory list failed", "error", err)
	} else {
		for k := range m {
			names[k] = struct{}{}
		}
	}
	if uc.environmentRepo != nil {
		if m, err := uc.environmentRepo.ListEnvironments(ctx); err != nil {
			slog.Warn("collectEnvironmentNames: etcd environments list failed", "error", err)
		} else {
			for k := range m {
				names[k] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(names))
	for k := range names {
		out = append(out, k)
	}
	return out
}

func (uc *APIServerSyncUseCase) environmentWithSnapshotsFromSchema(ctx context.Context, src *models.Environment) *models.Environment {
	out := &models.Environment{
		Name:      src.Name,
		Type:      src.Type,
		Bundles:   src.Bundles,
		Services:  src.Services,
		Snapshots: nil,
	}
	for _, bundle := range src.Bundles.Static {
		snaps, err := uc.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			slog.Warn("ListContractSnapshots failed", "environment", src.Name, "repository", bundle.Repository, "ref", bundle.Ref, "path", bundle.Path, "error", err)
			continue
		}
		out.Snapshots = append(out.Snapshots, snaps...)
	}
	return out
}

// StartEtcdFollowerWatch rebuilds xDS from controller etcd when the leader (or another writer) changes data.
// Every replica runs this so snapshots stay aligned without each one streaming from API Server.
//
// etcd Watch does not replay existing keys: without an initial rebuild, followers (and configs with
// environments only in etcd) would serve an empty xDS cache until the next write.
func (uc *APIServerSyncUseCase) StartEtcdFollowerWatch(ctx context.Context) {
	if uc.etcdClient == nil {
		slog.Warn("StartEtcdFollowerWatch: etcd client is nil")
		return
	}

	go func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := uc.rebuildAllXDS(flushCtx); err != nil {
			slog.Error("initial xDS rebuild from etcd (HA / cold start)", "error", err)
		} else {
			slog.Info("initial xDS rebuild from etcd completed")
		}
	}()

	ch := uc.etcdClient.Watch(ctx, controllerEtcdWatchPrefix, clientv3.WithPrefix())

	var mu sync.Mutex
	var debounce *time.Timer

	flush := func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := uc.rebuildAllXDS(flushCtx); err != nil {
			slog.Error("etcd follower watch: rebuild xDS", "error", err)
		}
	}

	for wresp := range ch {
		if err := wresp.Err(); err != nil {
			slog.Warn("controller etcd watch error", "error", err)
			continue
		}
		if len(wresp.Events) == 0 {
			continue
		}

		mu.Lock()
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(400*time.Millisecond, flush)
		mu.Unlock()
	}
	slog.Info("controller etcd watch channel closed")
}

func (uc *APIServerSyncUseCase) rebuildAllXDS(ctx context.Context) error {
	names := uc.collectEnvironmentNames(ctx)
	for _, name := range names {
		if err := uc.updateXDSSnapshot(ctx, name); err != nil {
			slog.Warn("rebuildAllXDS: environment", "name", name, "error", err)
		}
	}
	return nil
}

func sharedToControllerSnapshot(s sharedgit.ContractSnapshot) *models.ContractSnapshot {
	apps := make([]models.App, len(s.Access.Apps))
	for i, a := range s.Access.Apps {
		apps[i] = models.App{AppID: a.AppID, Environments: a.Environments}
	}
	return &models.ContractSnapshot{
		Name:                  s.Name,
		Prefix:                s.Prefix,
		Upstream:              models.ContractUpstream{Name: s.Upstream.Name},
		AllowUndefinedMethods: s.AllowUndefinedMethods,
		Access: models.Access{
			Secure: s.Access.Secure,
			Apps:   apps,
		},
	}
}
