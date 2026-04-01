package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type ControllerRegistryUseCase struct {
	controllerRepo interfaces.ControllerRepository
	snapshotRepo   interfaces.SnapshotRepository

	mu                sync.RWMutex
	controllerStreams map[string]interfaces.SnapshotStream
}

func NewControllerRegistryUseCase(
	controllerRepo interfaces.ControllerRepository,
	snapshotRepo interfaces.SnapshotRepository,
) *ControllerRegistryUseCase {
	return &ControllerRegistryUseCase{
		controllerRepo:    controllerRepo,
		snapshotRepo:      snapshotRepo,
		controllerStreams: make(map[string]interfaces.SnapshotStream),
	}
}

func (uc *ControllerRegistryUseCase) RegisterController(ctx context.Context, info models.ControllerInfo) error {
	slog.Info("Registering controller", "controller_id", info.ControllerID, "tenant", info.Tenant)

	if err := uc.controllerRepo.RegisterController(ctx, info); err != nil {
		return fmt.Errorf("failed to register controller: %w", err)
	}

	return nil
}

func (uc *ControllerRegistryUseCase) StreamSnapshots(ctx context.Context, controllerID string, stream interfaces.SnapshotStream) error {
	slog.Info("Starting snapshot stream", "controller_id", controllerID)

	uc.mu.Lock()
	uc.controllerStreams[controllerID] = stream
	uc.mu.Unlock()

	defer func() {
		uc.mu.Lock()
		delete(uc.controllerStreams, controllerID)
		uc.mu.Unlock()
	}()

	controller, err := uc.controllerRepo.GetController(ctx, controllerID)
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	slog.Info("Controller info", "controller_id", controllerID, "environments_count", len(controller.Environments))

	for _, env := range controller.Environments {
		slog.Info("Processing environment", "name", env.Name, "bundles_count", len(env.Bundles))
		for _, bundle := range env.Bundles {
			safeRef := strings.ReplaceAll(bundle.Ref, "/", "%2F")
			safePath := ""
			if bundle.Path == "" {
				safePath = "."
			} else {
				safePath = strings.ReplaceAll(bundle.Path, "/", "%2F")
			}
			bundleKey := fmt.Sprintf("%s/%s/%s", bundle.Repository, safeRef, safePath)

			slog.Info("Getting snapshots for bundle", "environment", env.Name, "bundle_key", bundleKey)

			snapshots, err := uc.snapshotRepo.GetSnapshots(ctx, bundleKey)
			if err != nil {
				slog.Error("Failed to get snapshots", "bundle_key", bundleKey, "error", err)
				continue
			}

			slog.Info("Sending snapshots to controller", "environment", env.Name, "bundle_key", bundleKey, "count", len(snapshots))

			if err := stream.Send(env.Name, bundleKey, snapshots); err != nil {
				return fmt.Errorf("failed to send snapshots: %w", err)
			}
		}
	}

	<-ctx.Done()
	return nil
}

func (uc *ControllerRegistryUseCase) Heartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo) error {
	slog.Debug("Received heartbeat", "controller_id", controllerID)

	// Get current controller information
	controller, err := uc.controllerRepo.GetController(ctx, controllerID)
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	// Check if the list of environments has changed
	hasChanges := false
	if len(controller.Environments) != len(environments) {
		hasChanges = true
	} else {
		// Create a map for fast search
		existingEnvs := make(map[string]bool)
		for _, env := range controller.Environments {
			existingEnvs[env.Name] = true
		}

		for _, env := range environments {
			if !existingEnvs[env.Name] {
				hasChanges = true
				break
			}
		}
	}

	// Update heartbeat
	if err := uc.controllerRepo.UpdateControllerHeartbeat(ctx, controllerID, environments); err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	// If there are changes, send snapshots for new environments to active stream
	if hasChanges {
		slog.Info("Detected environment changes, sending snapshots to stream", "controller_id", controllerID)

		uc.mu.RLock()
		stream, exists := uc.controllerStreams[controllerID]
		uc.mu.RUnlock()

		if exists {
			// Send snapshots for all environments (including new ones)
			for _, env := range environments {
				for _, bundle := range env.Bundles {
					safeRef := strings.ReplaceAll(bundle.Ref, "/", "%2F")
					safePath := ""
					if bundle.Path == "" {
						safePath = "."
					} else {
						safePath = strings.ReplaceAll(bundle.Path, "/", "%2F")
					}
					bundleKey := fmt.Sprintf("%s/%s/%s", bundle.Repository, safeRef, safePath)

					slog.Info("Getting snapshots for new environment", "environment", env.Name, "bundle_key", bundleKey)

					snapshots, err := uc.snapshotRepo.GetSnapshots(ctx, bundleKey)
					if err != nil {
						slog.Error("Failed to get snapshots for new environment", "error", err, "bundle_key", bundleKey)
						continue
					}

					slog.Info("Sending snapshots for new environment", "environment", env.Name, "bundle_key", bundleKey, "count", len(snapshots))

					if err := stream.Send(env.Name, bundleKey, snapshots); err != nil {
						slog.Error("Failed to send snapshots for new environment", "error", err)
					}
				}
			}
		}
	}

	return nil
}

func (uc *ControllerRegistryUseCase) NotifySnapshotUpdate(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	slog.Info("Notifying controllers about snapshot update", "bundle_key", bundleKey, "snapshots_count", len(snapshots))

	controllers, err := uc.controllerRepo.ListControllers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list controllers: %w", err)
	}

	for _, controller := range controllers {
		for _, env := range controller.Environments {
			for _, bundle := range env.Bundles {
				// Create bundleKey with the same formatting as in StreamSnapshots
				safeRef := strings.ReplaceAll(bundle.Ref, "/", "%2F")
				safePath := ""
				if bundle.Path == "" {
					safePath = "."
				} else {
					safePath = strings.ReplaceAll(bundle.Path, "/", "%2F")
				}
				currentBundleKey := fmt.Sprintf("%s/%s/%s", bundle.Repository, safeRef, safePath)

				if currentBundleKey == bundleKey {
					slog.Info("Found matching bundle for controller", "controller_id", controller.ControllerID, "environment", env.Name, "bundle_key", bundleKey)
					stream, ok := uc.controllerStreams[controller.ControllerID]
					if ok {
						slog.Info("Sending snapshot update to controller", "controller_id", controller.ControllerID, "environment", env.Name, "snapshots_count", len(snapshots))
						if err := stream.Send(env.Name, bundleKey, snapshots); err != nil {
							slog.Error("Failed to send snapshot update", "controller_id", controller.ControllerID, "error", err)
						}
					} else {
						slog.Warn("No active stream for controller", "controller_id", controller.ControllerID)
					}
				}
			}
		}
	}

	return nil
}
