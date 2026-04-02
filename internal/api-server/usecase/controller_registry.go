package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/domain/models"
	"merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "merionyx/api-gateway/internal/shared/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const apiServerEtcdPrefix = "/api-gateway/api-server/"

// ControllerRegistryUseCase registers controllers in etcd and serves StreamSnapshots.
//
// HA: one gRPC stream lives on whichever API Server replica accepted the connection (sticky LB per
// controller source IP). Every replica watches etcd; NotifySnapshotUpdate only delivers to streams
// registered on that process. Bundle writes to etcd come from the API Server leader only.
type ControllerRegistryUseCase struct {
	controllerRepo interfaces.ControllerRepository
	snapshotRepo   interfaces.SnapshotRepository
	etcdClient     *clientv3.Client

	mu                sync.RWMutex
	controllerStreams map[string]interfaces.SnapshotStream
}

func NewControllerRegistryUseCase(
	controllerRepo interfaces.ControllerRepository,
	snapshotRepo interfaces.SnapshotRepository,
	etcdClient *clientv3.Client,
) *ControllerRegistryUseCase {
	return &ControllerRegistryUseCase{
		controllerRepo:    controllerRepo,
		snapshotRepo:      snapshotRepo,
		etcdClient:        etcdClient,
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

func (uc *ControllerRegistryUseCase) sendAllSnapshotsForControllerStream(
	ctx context.Context,
	controllerID string,
	stream interfaces.SnapshotStream,
) error {
	controller, err := uc.controllerRepo.GetController(ctx, controllerID)
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	slog.Info("Sending snapshots to controller stream", "controller_id", controllerID, "environments_count", len(controller.Environments))

	for _, env := range controller.Environments {
		for _, bundle := range env.Bundles {
			bundleKey := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)

			snapshots, err := uc.snapshotRepo.GetSnapshots(ctx, bundleKey)
			if err != nil {
				slog.Error("Failed to get snapshots", "bundle_key", bundleKey, "error", err)
				continue
			}

			if err := stream.Send(env.Name, bundleKey, snapshots); err != nil {
				return fmt.Errorf("failed to send snapshots: %w", err)
			}
		}
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

	if err := uc.sendAllSnapshotsForControllerStream(ctx, controllerID, stream); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func (uc *ControllerRegistryUseCase) Heartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo) error {
	slog.Debug("Received heartbeat", "controller_id", controllerID)

	controller, err := uc.controllerRepo.GetController(ctx, controllerID)
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	hasChanges := controllerEnvironmentsChanged(controller.Environments, environments)

	if err := uc.controllerRepo.UpdateControllerHeartbeat(ctx, controllerID, environments); err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	if hasChanges {
		slog.Info("Detected environment changes, sending snapshots to stream", "controller_id", controllerID)

		uc.mu.RLock()
		stream, exists := uc.controllerStreams[controllerID]
		uc.mu.RUnlock()

		if exists {
			if err := uc.sendAllSnapshotsForControllerStream(ctx, controllerID, stream); err != nil {
				slog.Error("Failed to send snapshots after env change", "error", err)
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
				currentBundleKey := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)
				if currentBundleKey != bundleKey {
					continue
				}
				stream, ok := uc.controllerStreams[controller.ControllerID]
				if !ok {
					// Expected on API Server replicas that do not hold this controller's gRPC stream (HA).
					slog.Debug("No active stream for controller on this replica", "controller_id", controller.ControllerID)
					continue
				}
				slog.Info("Sending snapshot update to controller", "controller_id", controller.ControllerID, "environment", env.Name, "snapshots_count", len(snapshots))
				if err := stream.Send(env.Name, bundleKey, snapshots); err != nil {
					slog.Error("Failed to send snapshot update", "controller_id", controller.ControllerID, "error", err)
				}
			}
		}
	}

	return nil
}

// StartEtcdWatch watches API Server etcd and pushes updates to gRPC streams on this instance
// (so any replica receives the same etcd events and notifies only its connected controllers).
func (uc *ControllerRegistryUseCase) StartEtcdWatch(ctx context.Context) {
	if uc.etcdClient == nil {
		slog.Warn("StartEtcdWatch: etcd client is nil, skipping")
		return
	}

	ch := uc.etcdClient.Watch(ctx, apiServerEtcdPrefix, clientv3.WithPrefix())

	pendingBundles := make(map[string]struct{})
	pendingControllers := make(map[string]struct{})

	var mu sync.Mutex
	var debounce *time.Timer

	flush := func() {
		mu.Lock()
		bundles := pendingBundles
		controllers := pendingControllers
		pendingBundles = make(map[string]struct{})
		pendingControllers = make(map[string]struct{})
		mu.Unlock()

		flushCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		for bk := range bundles {
			snapshots, err := uc.snapshotRepo.GetSnapshots(flushCtx, bk)
			if err != nil {
				slog.Error("etcd watch: reload snapshots", "bundle_key", bk, "error", err)
				continue
			}
			if err := uc.NotifySnapshotUpdate(flushCtx, bk, snapshots); err != nil {
				slog.Error("etcd watch: notify snapshot", "bundle_key", bk, "error", err)
			}
		}
		for cid := range controllers {
			if err := uc.resyncConnectedController(flushCtx, cid); err != nil {
				slog.Error("etcd watch: resync controller", "controller_id", cid, "error", err)
			}
		}
	}

	for wresp := range ch {
		if err := wresp.Err(); err != nil {
			slog.Warn("API Server etcd watch error", "error", err)
			continue
		}
		for _, ev := range wresp.Events {
			if ev.Kv == nil {
				continue
			}
			key := string(ev.Kv.Key)
			mu.Lock()
			if bk, ok := parseBundleKeyFromSnapshotKey(key); ok {
				pendingBundles[bk] = struct{}{}
			} else if cid, ok := parseControllerIDKey(key); ok {
				pendingControllers[cid] = struct{}{}
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(300*time.Millisecond, flush)
			mu.Unlock()
		}
	}
	slog.Info("API Server etcd watch channel closed")
}

func parseBundleKeyFromSnapshotKey(key string) (bundleKey string, ok bool) {
	const p = "/api-gateway/api-server/snapshots/"
	if !strings.HasPrefix(key, p) {
		return "", false
	}
	rest := strings.TrimPrefix(key, p)
	idx := strings.Index(rest, "/contracts/")
	if idx < 0 {
		return "", false
	}
	return rest[:idx], true
}

func parseControllerIDKey(key string) (controllerID string, ok bool) {
	const p = "/api-gateway/api-server/controllers/"
	if !strings.HasPrefix(key, p) {
		return "", false
	}
	rest := strings.TrimPrefix(key, p)
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return rest, true
}

// controllerEnvironmentsChanged is true when env names or bundle sets (repository/ref/path) differ.
func controllerEnvironmentsChanged(stored, incoming []models.EnvironmentInfo) bool {
	incomingByName := make(map[string][]models.BundleInfo, len(incoming))
	for _, e := range incoming {
		incomingByName[e.Name] = e.Bundles
	}
	if len(stored) != len(incomingByName) {
		return true
	}
	for _, e := range stored {
		bundlesIn, ok := incomingByName[e.Name]
		if !ok || !bundleMultisetsEqual(e.Bundles, bundlesIn) {
			return true
		}
	}
	return false
}

func bundleMultisetsEqual(a, b []models.BundleInfo) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, x := range a {
		counts[bundlekey.Build(x.Repository, x.Ref, x.Path)]++
	}
	for _, x := range b {
		k := bundlekey.Build(x.Repository, x.Ref, x.Path)
		counts[k]--
		if counts[k] < 0 {
			return false
		}
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}

func (uc *ControllerRegistryUseCase) resyncConnectedController(ctx context.Context, controllerID string) error {
	uc.mu.RLock()
	stream, ok := uc.controllerStreams[controllerID]
	uc.mu.RUnlock()
	if !ok {
		return nil
	}
	return uc.sendAllSnapshotsForControllerStream(ctx, controllerID, stream)
}
