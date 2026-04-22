package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/controller/xds/snapshot"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	xdsResource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

type snapshotsUseCase struct {
	environmentUseCase interfaces.EnvironmentsUseCase
	xdsSnapshotManager *cache.SnapshotManager
	xdsBuilder         interfaces.XDSBuilder
}

func NewSnapshotsUseCase() interfaces.SnapshotsUseCase {
	return &snapshotsUseCase{}
}

func (uc *snapshotsUseCase) SetDependencies(environmentUseCase interfaces.EnvironmentsUseCase, xdsSnapshotManager *cache.SnapshotManager, xdsBuilder interfaces.XDSBuilder) {
	uc.environmentUseCase = environmentUseCase
	uc.xdsSnapshotManager = xdsSnapshotManager
	uc.xdsBuilder = xdsBuilder
}

func (uc *snapshotsUseCase) UpdateSnapshot(ctx context.Context, req *models.UpdateSnapshotRequest) (*models.UpdateSnapshotResponse, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseControllerPkg, "UpdateSnapshot"))
	defer span.End()
	var updatedEnvironments []string

	if req.Environment == "" {
		// Update all environments
		environments, err := uc.environmentUseCase.ListEnvironments(ctx)
		if err != nil {
			telemetry.MarkError(span, err)
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}

		for envName, env := range environments {
			if err := uc.rebuildSnapshot(ctx, envName, env); err != nil {
				slog.Warn("failed to rebuild xDS snapshot", "environment", envName, "error", err)
			} else {
				updatedEnvironments = append(updatedEnvironments, envName)
			}
		}
	} else {
		// Update specific environment
		env, err := uc.environmentUseCase.GetEnvironment(ctx, req.Environment)
		if err != nil {
			telemetry.MarkError(span, err)
			return nil, fmt.Errorf("environment %s not found: %w", req.Environment, err)
		}

		if err := uc.rebuildSnapshot(ctx, req.Environment, env); err != nil {
			telemetry.MarkError(span, err)
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
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseControllerPkg, "GetSnapshotStatus"))
	defer span.End()
	env, err := uc.environmentUseCase.GetEnvironment(ctx, req.Environment)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, fmt.Errorf("environment %s not found: %w", req.Environment, err)
	}

	nodeID := fmt.Sprintf("envoy-%s", req.Environment)
	xdsSnapshot, err := uc.xdsSnapshotManager.GetSnapshot(nodeID)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, fmt.Errorf("failed to get xDS snapshot: %w", err)
	}

	// Count resources
	clustersCount := len(xdsSnapshot.GetResources(xdsResource.ClusterType))
	routesCount := len(xdsSnapshot.GetResources(xdsResource.RouteType))

	return &models.GetSnapshotStatusResponse{
		Environment:    req.Environment,
		Version:        xdsSnapshot.GetVersion(xdsResource.ClusterType),
		ContractsCount: int32(len(env.Snapshots)),
		ClustersCount:  int32(clustersCount),
		RoutesCount:    int32(routesCount),
	}, nil
}

func (uc *snapshotsUseCase) rebuildSnapshot(ctx context.Context, envName string, env *models.Environment) error {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseControllerPkg, "rebuildSnapshot"))
	defer span.End()
	xdsSnapshot, err := snapshot.BuildEnvoySnapshot(uc.xdsBuilder, env)
	if err != nil {
		telemetry.MarkError(span, err)
		return fmt.Errorf("build envoy snapshot: %w", err)
	}
	nodeID := fmt.Sprintf("envoy-%s", envName)

	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnapshot); err != nil {
		telemetry.MarkError(span, err)
		return fmt.Errorf("failed to update xDS snapshot: %w", err)
	}

	slog.Info("xDS snapshot rebuilt", "environment", envName, "node_id", nodeID)
	return nil
}
