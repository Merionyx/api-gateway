package usecase

import (
	"context"
	"log/slog"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const controllerEtcdWatchPrefix = "/api-gateway/controller/"

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
