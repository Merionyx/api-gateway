package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/controller/xds/snapshot"
)

type environmentsUseCase struct {
	environmentRepo     interfaces.EnvironmentRepository
	inMemoryEnvs        interfaces.InMemoryEnvironmentsRepository
	schemasUseCase      interfaces.SchemasUseCase
	xdsSnapshotManager  *xdscache.SnapshotManager
	xdsBuilder          interfaces.XDSBuilder
}

func NewEnvironmentsUseCase() interfaces.EnvironmentsUseCase {
	return &environmentsUseCase{}
}

func (uc *environmentsUseCase) SetDependencies(environmentRepo interfaces.EnvironmentRepository, inMemory interfaces.InMemoryEnvironmentsRepository, schemasUseCase interfaces.SchemasUseCase, xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder interfaces.XDSBuilder) {
	uc.environmentRepo = environmentRepo
	uc.inMemoryEnvs = inMemory
	uc.schemasUseCase = schemasUseCase
	uc.xdsSnapshotManager = xdsSnapshotManager
	uc.xdsBuilder = xdsBuilder
}

func (uc *environmentsUseCase) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment does not exist
	existing, _ := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("environment %s already exists", req.Name)
	}

	env := &models.Environment{
		Name:      req.Name,
		Type:      req.Type,
		Bundles:   req.Bundles,
		Services:  req.Services,
		Snapshots: make([]models.ContractSnapshot, 0),
	}

	// Save to etcd
	if err := uc.environmentRepo.SaveEnvironment(ctx, env); err != nil {
		return nil, fmt.Errorf("failed to save environment: %w", err)
	}

	slog.Info("environment created", "name", req.Name)

	if err := uc.rebuildEffectiveXDSSnapshot(ctx, req.Name); err != nil {
		slog.Warn("failed to build initial xDS snapshot", "environment", req.Name, "error", err)
	}

	return env, nil
}

func (uc *environmentsUseCase) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	env, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	for _, bundle := range env.Bundles.Static {
		snapshots, err := uc.schemasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get environment snapshots: %w", err)
		}
		env.Snapshots = append(env.Snapshots, snapshots...)
	}

	return env, nil
}

func (uc *environmentsUseCase) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	environments, err := uc.environmentRepo.ListEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	for _, environment := range environments {
		for _, bundle := range environment.Bundles.Static {
			snapshots, err := uc.schemasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to get environment snapshots: %w", err)
			}
			environment.Snapshots = append(environment.Snapshots, snapshots...)
		}
		environments[environment.Name] = environment
	}
	return environments, nil
}

func (uc *environmentsUseCase) UpdateEnvironment(ctx context.Context, req *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment exists
	existing, err := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %w", req.Name, err)
	}

	// Update fields
	existing.Bundles = req.Bundles
	existing.Services = req.Services

	// Save to etcd
	if err := uc.environmentRepo.SaveEnvironment(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update environment: %w", err)
	}

	slog.Info("environment updated", "name", req.Name)

	if err := uc.rebuildEffectiveXDSSnapshot(ctx, req.Name); err != nil {
		slog.Warn("failed to rebuild xDS snapshot", "environment", req.Name, "error", err)
	}

	return existing, nil
}

func (uc *environmentsUseCase) DeleteEnvironment(ctx context.Context, name string) error {
	// Check if environment exists
	_, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return fmt.Errorf("environment %s not found: %w", name, err)
	}

	// Delete from etcd
	if err := uc.environmentRepo.DeleteEnvironment(ctx, name); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	slog.Info("environment deleted", "name", name)

	// Delete xDS snapshot (optional, depends on your implementation)
	nodeID := fmt.Sprintf("envoy-%s", name)
	if err := uc.xdsSnapshotManager.DeleteSnapshot(nodeID); err != nil {
		return fmt.Errorf("delete xds snapshot: %w", err)
	}

	return nil
}

// rebuildEffectiveXDSSnapshot uses the same mem ∪ etcd effective merge as APIServerSync / memory xDS.
func (uc *environmentsUseCase) rebuildEffectiveXDSSnapshot(ctx context.Context, name string) error {
	var mem, etcdE *models.Environment
	if uc.inMemoryEnvs != nil {
		if m, err := uc.inMemoryEnvs.GetEnvironment(ctx, name); err == nil {
			mem = m
		}
	}
	if uc.environmentRepo != nil {
		if e, err := uc.environmentRepo.GetEnvironment(ctx, name); err == nil {
			etcdE = e
		}
	}
	skel, err := envmodel.BuildOptionalEffectiveEnvironment(mem, etcdE)
	if err != nil {
		return fmt.Errorf("build effective: %w", err)
	}
	var snaps []models.ContractSnapshot
	if skel.Bundles != nil {
		for _, b := range skel.Bundles.Static {
			list, err := uc.schemasUseCase.ListContractSnapshots(ctx, b.Repository, b.Ref, b.Path)
			if err != nil {
				slog.Warn("ListContractSnapshots in xDS rebuild", "environment", name, "repository", b.Repository, "ref", b.Ref, "path", b.Path, "error", err)
				continue
			}
			snaps = append(snaps, list...)
		}
	}
	env := &models.Environment{
		Name:      skel.Name,
		Type:      skel.Type,
		Bundles:   skel.Bundles,
		Services:  skel.Services,
		Snapshots: snaps,
	}
	xdsSnap, err := snapshot.BuildEnvoySnapshot(uc.xdsBuilder, env)
	if err != nil {
		return fmt.Errorf("build envoy snapshot: %w", err)
	}
	nodeID := fmt.Sprintf("envoy-%s", name)
	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnap); err != nil {
		return fmt.Errorf("failed to update xDS snapshot: %w", err)
	}
	slog.Info("xDS snapshot rebuilt", "environment", name, "reconcile", "memory_etcd_merged")
	return nil
}
