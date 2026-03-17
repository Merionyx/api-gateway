package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/xds/cache"
	"merionyx/api-gateway/control-plane/internal/xds/snapshot"

	xdsResource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

type snapshotsUseCase struct {
	environmentRepo    interfaces.EnvironmentRepository
	xdsSnapshotManager *cache.SnapshotManager
}

func NewSnapshotsUseCase(
	environmentRepo interfaces.EnvironmentRepository,
	xdsSnapshotManager *cache.SnapshotManager,
) interfaces.SnapshotsUseCase {
	return &snapshotsUseCase{
		environmentRepo:    environmentRepo,
		xdsSnapshotManager: xdsSnapshotManager,
	}
}

func (uc *snapshotsUseCase) UpdateSnapshot(ctx context.Context, req *models.UpdateSnapshotRequest) (*models.UpdateSnapshotResponse, error) {
	var updatedEnvironments []string

	if req.Environment == "" {
		// Update all environments
		environments, err := uc.environmentRepo.ListEnvironments(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}

		for envName, env := range environments {
			if err := uc.rebuildSnapshot(ctx, envName, env); err != nil {
				log.Printf("Failed to rebuild snapshot for %s: %v", envName, err)
			} else {
				updatedEnvironments = append(updatedEnvironments, envName)
			}
		}
	} else {
		// Update specific environment
		env, err := uc.environmentRepo.GetEnvironment(ctx, req.Environment)
		if err != nil {
			return nil, fmt.Errorf("environment %s not found: %w", req.Environment, err)
		}

		if err := uc.rebuildSnapshot(ctx, req.Environment, env); err != nil {
			return nil, fmt.Errorf("failed to rebuild snapshot: %w", err)
		}
		updatedEnvironments = append(updatedEnvironments, req.Environment)
	}

	return &models.UpdateSnapshotResponse{
		Success:             true,
		UpdatedEnvironments: updatedEnvironments,
	}, nil
}

func (uc *snapshotsUseCase) GetSnapshotStatus(ctx context.Context, req *models.GetSnapshotStatusRequest) (*models.GetSnapshotStatusResponse, error) {
	env, err := uc.environmentRepo.GetEnvironment(ctx, req.Environment)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %w", req.Environment, err)
	}

	nodeID := fmt.Sprintf("envoy-%s", req.Environment)
	xdsSnapshot, err := uc.xdsSnapshotManager.GetSnapshot(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get xDS snapshot: %w", err)
	}

	// Count resources
	clustersCount := len(xdsSnapshot.GetResources(xdsResource.ClusterType))
	routesCount := len(xdsSnapshot.GetResources(xdsResource.RouteType))

	return &models.GetSnapshotStatusResponse{
		Environment:    req.Environment,
		Version:        xdsSnapshot.GetVersion(xdsResource.ClusterType),
		LastUpdated:    time.Now().Unix(), // TODO: store real update time
		ContractsCount: int32(len(env.Snapshots)),
		ClustersCount:  int32(clustersCount),
		RoutesCount:    int32(routesCount),
	}, nil
}

func (uc *snapshotsUseCase) rebuildSnapshot(ctx context.Context, envName string, env *models.Environment) error {
	xdsSnapshot := snapshot.BuildEnvoySnapshot(env)
	nodeID := fmt.Sprintf("envoy-%s", envName)

	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnapshot); err != nil {
		return fmt.Errorf("failed to update xDS snapshot: %w", err)
	}

	log.Printf("xDS snapshot rebuilt for environment: %s (nodeID: %s)", envName, nodeID)
	return nil
}
