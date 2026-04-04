package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ctrlmetrics "merionyx/api-gateway/internal/controller/metrics"

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
		start := time.Now()
		err := uc.rebuildAllXDS(flushCtx)
		uc.recordRebuildMetrics(ctrlmetrics.RebuildPhaseInitial, err, time.Since(start))
		if err != nil {
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
		start := time.Now()
		err := uc.rebuildAllXDS(flushCtx)
		uc.recordRebuildMetrics(ctrlmetrics.RebuildPhaseDebounced, err, time.Since(start))
		if err != nil {
			slog.Error("etcd follower watch: rebuild xDS", "error", err)
		}
	}

	en := uc.config.MetricsHTTP.Enabled
	for wresp := range ch {
		if err := wresp.Err(); err != nil {
			ctrlmetrics.RecordEtcdWatchError(en)
			slog.Warn("controller etcd watch error", "error", err)
			continue
		}
		if n := len(wresp.Events); n > 0 {
			ctrlmetrics.AddEtcdWatchEvents(en, n)
		} else {
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
	var nFail int
	for _, name := range names {
		if err := uc.updateXDSSnapshot(ctx, name); err != nil {
			nFail++
			slog.Warn("rebuildAllXDS: environment", "name", name, "error", err)
		}
	}
	if nFail > 0 {
		return fmt.Errorf("rebuildAllXDS: %d of %d environments failed", nFail, len(names))
	}
	return nil
}

func (uc *APIServerSyncUseCase) recordRebuildMetrics(phase string, err error, d time.Duration) {
	en := uc.config.MetricsHTTP.Enabled
	res := ctrlmetrics.XDSResultOK
	if err != nil {
		res = ctrlmetrics.XDSResultError
	}
	ctrlmetrics.RecordXDSRebuildFlush(en, phase, res)
	ctrlmetrics.ObserveXDSRebuildDuration(en, phase, d)
}
