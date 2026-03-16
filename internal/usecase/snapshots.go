package usecase

import (
	"context"
	"fmt"
	"log"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/xds/cache"
	"merionyx/api-gateway/control-plane/internal/xds/snapshot"
)

type snapshotsUseCase struct {
	environments       *map[string]*models.Environment
	XDSSnapshotManager *cache.SnapshotManager
}

// NewSnapshotsUseCase creates a new instance of SnapshotsUseCase
func NewSnapshotsUseCase(environments *map[string]*models.Environment, xDSSnapshotManager *cache.SnapshotManager) interfaces.SnapshotsUseCase {
	return &snapshotsUseCase{environments: environments, XDSSnapshotManager: xDSSnapshotManager}
}

func (uc *snapshotsUseCase) UpdateSnapshot(ctx context.Context, req *models.UpdateSnapshotRequest) (*models.UpdateSnapshotResponse, error) {
	log.Println("Updating snapshot for environment:", req.Environment)
	log.Println("Environments:", uc.environments)
	log.Println("XDSSnapshotManager:", uc.XDSSnapshotManager)
	log.Println("UC:", uc)

	if req.Environment == "" {
		for envName, env := range *uc.environments {
			snapshot := snapshot.BuildEnvoySnapshot(env)
			nodeID := fmt.Sprintf("envoy-%s", envName)

			if err := uc.XDSSnapshotManager.UpdateSnapshot(nodeID, snapshot); err != nil {
				log.Fatalf("Failed to update snapshot for %s: %v", nodeID, err)
			}

			log.Printf("Created xDS snapshot for environment: %s (nodeID: %s)", envName, nodeID)
		}
	} else {
		env := (*uc.environments)[req.Environment]
		snapshot := snapshot.BuildEnvoySnapshot(env)
		nodeID := fmt.Sprintf("envoy-%s", req.Environment)

		if err := uc.XDSSnapshotManager.UpdateSnapshot(nodeID, snapshot); err != nil {
			log.Fatalf("Failed to update snapshot for %s: %v", nodeID, err)
		}

		log.Printf("Created xDS snapshot for environment: %s (nodeID: %s)", req.Environment, nodeID)
	}

	return &models.UpdateSnapshotResponse{Success: true}, nil
}
