package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ctrlmetrics "merionyx/api-gateway/internal/controller/metrics"
	"merionyx/api-gateway/internal/controller/repository/cache"
	ctrlrepoetcd "merionyx/api-gateway/internal/controller/repository/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sync/semaphore"
)

const xdsRebuildParallelism = 8

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

	ch := uc.etcdClient.Watch(ctx, ctrlrepoetcd.ControllerWatchPrefix, clientv3.WithPrefix())

	var mu sync.Mutex
	var debounce *time.Timer
	dirtyEnvs := make(map[string]struct{})
	var needFull bool

	flush := func() {
		mu.Lock()
		toFlush := dirtyEnvs
		full := needFull
		dirtyEnvs = make(map[string]struct{})
		needFull = false
		mu.Unlock()

		if !full && len(toFlush) == 0 {
			return
		}

		flushCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		start := time.Now()
		var err error
		if full {
			err = uc.rebuildAllXDS(flushCtx)
		} else {
			names := make([]string, 0, len(toFlush))
			for e := range toFlush {
				names = append(names, e)
			}
			err = uc.rebuildXDSForEnvironments(flushCtx, names)
		}
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

		var batchTouched bool
		for _, event := range wresp.Events {
			if event.Kv == nil {
				continue
			}
			key := string(event.Kv.Key)
			eff := cache.ClassifyControllerEtcdWatchKey(key)
			if eff.Ignore {
				continue
			}

			if eff.SchemaBundleKey != "" {
				batchTouched = true
				if uc.schemaCache != nil {
					uc.schemaCache.InvalidateBundleKey(eff.SchemaBundleKey)
				}
				envNames := uc.environmentsForBundleKey(context.Background(), eff.SchemaBundleKey)
				if len(envNames) == 0 {
					mu.Lock()
					needFull = true
					mu.Unlock()
					continue
				}
				mu.Lock()
				for _, envName := range envNames {
					dirtyEnvs[envName] = struct{}{}
				}
				mu.Unlock()
				continue
			}

			if eff.Environment != "" {
				batchTouched = true
				if uc.bundleEnvIndex != nil {
					rctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					uc.bundleEnvIndex.Rebuild(rctx)
					cancel()
					ctrlmetrics.RecordBundleEnvIndexRebuild(en)
				}
				mu.Lock()
				dirtyEnvs[eff.Environment] = struct{}{}
				mu.Unlock()
				continue
			}

			if eff.UnknownUnderPrefix {
				batchTouched = true
				mu.Lock()
				needFull = true
				mu.Unlock()
			}
		}

		if !batchTouched {
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

func (uc *APIServerSyncUseCase) environmentsForBundleKey(ctx context.Context, bundleKey string) []string {
	if uc.bundleEnvIndex == nil {
		return nil
	}
	envs := uc.bundleEnvIndex.EnvironmentsForBundle(bundleKey)
	if len(envs) > 0 {
		return envs
	}
	rctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	uc.bundleEnvIndex.Rebuild(rctx)
	cancel()
	ctrlmetrics.RecordBundleEnvIndexRebuild(uc.config.MetricsHTTP.Enabled)
	return uc.bundleEnvIndex.EnvironmentsForBundle(bundleKey)
}

func (uc *APIServerSyncUseCase) rebuildAllXDS(ctx context.Context) error {
	names := uc.collectEnvironmentNames(ctx)
	return uc.rebuildXDSForEnvironments(ctx, names)
}

func (uc *APIServerSyncUseCase) rebuildXDSForEnvironments(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	sem := semaphore.NewWeighted(xdsRebuildParallelism)
	var wg sync.WaitGroup
	var failMu sync.Mutex
	var nFail int
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				return
			}
			defer sem.Release(1)
			if err := uc.updateXDSSnapshot(ctx, name); err != nil {
				failMu.Lock()
				nFail++
				failMu.Unlock()
				slog.Warn("rebuildXDSForEnvironments: environment", "name", name, "error", err)
			}
		}()
	}
	wg.Wait()
	if nFail > 0 {
		return fmt.Errorf("rebuildXDSForEnvironments: %d of %d environments failed", nFail, len(names))
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
