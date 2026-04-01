package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	xdssnapshot "merionyx/api-gateway/internal/controller/xds/snapshot"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	pb "merionyx/api-gateway/pkg/api/controller_registry/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type APIServerSyncUseCase struct {
	config                   *config.Config
	schemaRepo               interfaces.SchemaRepository
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository
	environmentsUseCase      interfaces.EnvironmentsUseCase
	xdsSnapshotManager       *xdscache.SnapshotManager
	apiServerAddress         string
	controllerID             string
	xdsBuilder               interfaces.XDSBuilder
}

func NewAPIServerSyncUseCase(
	cfg *config.Config,
	schemaRepo interfaces.SchemaRepository,
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository,
	environmentsUseCase interfaces.EnvironmentsUseCase,
	xdsSnapshotManager *xdscache.SnapshotManager,
	xdsBuilder interfaces.XDSBuilder,
) *APIServerSyncUseCase {
	return &APIServerSyncUseCase{
		config:                   cfg,
		schemaRepo:               schemaRepo,
		inMemoryEnvironmentsRepo: inMemoryEnvironmentsRepo,
		environmentsUseCase:      environmentsUseCase,
		xdsSnapshotManager:       xdsSnapshotManager,
		apiServerAddress:         cfg.APIServer.Address,
		controllerID:             fmt.Sprintf("controller-%d", time.Now().Unix()),
		xdsBuilder:               xdsBuilder,
	}
}

func (uc *APIServerSyncUseCase) RegisterAndStream(ctx context.Context) error {
	slog.Info("Connecting to API Server", "address", uc.apiServerAddress)

	conn, err := grpc.NewClient(uc.apiServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to API Server: %w", err)
	}
	defer conn.Close()

	client := pb.NewControllerRegistryServiceClient(conn)

	if err := uc.registerController(ctx, client); err != nil {
		return fmt.Errorf("failed to register controller: %w", err)
	}

	go uc.startHeartbeat(ctx, client)

	if err := uc.streamSnapshots(ctx, client); err != nil {
		return fmt.Errorf("failed to stream snapshots: %w", err)
	}

	return nil
}

func (uc *APIServerSyncUseCase) registerController(ctx context.Context, client pb.ControllerRegistryServiceClient) error {
	// Collect environments from in-memory repository (config) and etcd
	environmentsMap := make(map[string]*pb.EnvironmentInfo)

	// 1. Add environments from in-memory repository (config)
	inMemoryEnvs, err := uc.inMemoryEnvironmentsRepo.ListEnvironments(ctx)
	if err != nil {
		slog.Warn("Failed to list in-memory environments", "error", err)
	} else {
		for _, env := range inMemoryEnvs {
			var bundles []*pb.BundleInfo
			for _, bundle := range env.Bundles.Static {
				bundles = append(bundles, &pb.BundleInfo{
					Name:       bundle.Name,
					Repository: bundle.Repository,
					Ref:        bundle.Ref,
					Path:       bundle.Path,
				})
			}
			environmentsMap[env.Name] = &pb.EnvironmentInfo{
				Name:    env.Name,
				Bundles: bundles,
			}
		}
	}

	// 2. Add/update environments from etcd (dynamically created)
	etcdEnvs, err := uc.environmentsUseCase.ListEnvironments(ctx)
	if err != nil {
		slog.Warn("Failed to list environments from etcd", "error", err)
	} else {
		for _, env := range etcdEnvs {
			var bundles []*pb.BundleInfo
			for _, bundle := range env.Bundles.Static {
				bundles = append(bundles, &pb.BundleInfo{
					Name:       bundle.Name,
					Repository: bundle.Repository,
					Ref:        bundle.Ref,
					Path:       bundle.Path,
				})
			}
			environmentsMap[env.Name] = &pb.EnvironmentInfo{
				Name:    env.Name,
				Bundles: bundles,
			}
		}
	}

	// 3. Convert map to slice
	var environments []*pb.EnvironmentInfo
	for _, env := range environmentsMap {
		environments = append(environments, env)
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
			// Collect environments from in-memory (config) and etcd
			environmentsMap := make(map[string]*pb.EnvironmentInfo)

			// 1. In-memory environments (config)
			inMemoryEnvs, err := uc.inMemoryEnvironmentsRepo.ListEnvironments(ctx)
			if err != nil {
				slog.Error("Failed to list in-memory environments for heartbeat", "error", err)
			} else {
				for _, env := range inMemoryEnvs {
					var bundles []*pb.BundleInfo
					for _, bundle := range env.Bundles.Static {
						bundles = append(bundles, &pb.BundleInfo{
							Name:       bundle.Name,
							Repository: bundle.Repository,
							Ref:        bundle.Ref,
							Path:       bundle.Path,
						})
					}
					environmentsMap[env.Name] = &pb.EnvironmentInfo{
						Name:    env.Name,
						Bundles: bundles,
					}
				}
			}

			// 2. Environments from etcd (dynamically created)
			etcdEnvs, err := uc.environmentsUseCase.ListEnvironments(ctx)
			if err != nil {
				slog.Error("Failed to list etcd environments for heartbeat", "error", err)
			} else {
				for _, env := range etcdEnvs {
					var bundles []*pb.BundleInfo
					for _, bundle := range env.Bundles.Static {
						bundles = append(bundles, &pb.BundleInfo{
							Name:       bundle.Name,
							Repository: bundle.Repository,
							Ref:        bundle.Ref,
							Path:       bundle.Path,
						})
					}
					environmentsMap[env.Name] = &pb.EnvironmentInfo{
						Name:    env.Name,
						Bundles: bundles,
					}
				}
			}

			// 3. Convert map to slice
			var environments []*pb.EnvironmentInfo
			for _, env := range environmentsMap {
				environments = append(environments, env)
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
		return err
	}

	slog.Info("Started receiving snapshot stream")

	for {
		resp, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		slog.Info("Received snapshots", "environment", resp.Environment, "bundle_key", resp.BundleKey, "count", len(resp.Snapshots))

		if len(resp.Snapshots) == 0 {
			slog.Debug("Skipping empty snapshot batch to avoid clearing xDS", "environment", resp.Environment, "bundle_key", resp.BundleKey)
			continue
		}

		var snapshots []sharedgit.ContractSnapshot
		for _, pbSnapshot := range resp.Snapshots {
			var apps []sharedgit.App
			for _, pbApp := range pbSnapshot.Access.Apps {
				apps = append(apps, sharedgit.App{
					AppID:        pbApp.AppId,
					Environments: pbApp.Environments,
				})
			}

			snapshots = append(snapshots, sharedgit.ContractSnapshot{
				Name:   pbSnapshot.Name,
				Prefix: pbSnapshot.Prefix,
				Upstream: sharedgit.ContractUpstream{
					Name: pbSnapshot.Upstream.Name,
				},
				AllowUndefinedMethods: pbSnapshot.AllowUndefinedMethods,
				Access: sharedgit.Access{
					Secure: pbSnapshot.Access.Secure,
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
	parts := strings.Split(bundleKey, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid bundle key format: %s", bundleKey)
	}

	repository := parts[0]
	refEscaped := parts[1]
	ref, err := url.PathUnescape(refEscaped)
	if err != nil {
		ref = refEscaped
	}

	for _, s := range snapshots {
		cs := sharedToControllerSnapshot(s)
		slog.Info("Saving snapshot to etcd", "repository", repository, "ref", ref, "contract", s.Name)
		if err := uc.schemaRepo.SaveContractSnapshot(ctx, repository, ref, s.Name, cs); err != nil {
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

	xdsSnap := xdssnapshot.BuildEnvoySnapshot(uc.xdsBuilder, env)
	nodeID := fmt.Sprintf("envoy-%s", environment)
	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnap); err != nil {
		return fmt.Errorf("failed to push xDS snapshot: %w", err)
	}
	return nil
}

func (uc *APIServerSyncUseCase) environmentForXDS(ctx context.Context, name string) (*models.Environment, error) {
	env, err := uc.environmentsUseCase.GetEnvironment(ctx, name)
	if err == nil {
		return env, nil
	}

	memEnv, memErr := uc.inMemoryEnvironmentsRepo.GetEnvironment(ctx, name)
	if memErr != nil {
		return nil, fmt.Errorf("environment %s: etcd %v; memory %w", name, err, memErr)
	}

	return uc.environmentWithSnapshotsFromSchema(ctx, memEnv), nil
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
		snaps, err := uc.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref)
		if err != nil {
			slog.Warn("ListContractSnapshots failed", "environment", src.Name, "repository", bundle.Repository, "ref", bundle.Ref, "error", err)
			continue
		}
		out.Snapshots = append(out.Snapshots, snaps...)
	}
	return out
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
