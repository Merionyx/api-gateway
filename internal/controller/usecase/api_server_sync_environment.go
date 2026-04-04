package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/controller/repository/memory"
	xdssnapshot "merionyx/api-gateway/internal/controller/xds/snapshot"
	"merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	pb "merionyx/api-gateway/pkg/api/controller_registry/v1"
)

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
