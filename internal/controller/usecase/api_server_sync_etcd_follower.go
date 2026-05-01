package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/controller/repository/cache"
	ctrlrepoetcd "github.com/merionyx/api-gateway/internal/controller/repository/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sync/semaphore"
)

const xdsRebuildParallelism = 8

// etcdFollowerWatch runs on every replica: initial xDS rebuild from controller etcd, then watch + debounced rebuilds.
type etcdFollowerWatch struct {
	config         *config.Config
	etcdClient     *clientv3.Client
	schemaCache    *cache.SchemaCache
	bundleEnvIndex *bundleenv.Index
	reconciler     interfaces.EffectiveReconciler
	reg            *registryEnvironmentsBuilder
}

func newEtcdFollowerWatch(
	cfg *config.Config,
	etcd *clientv3.Client,
	schemaCache *cache.SchemaCache,
	bundleIndex *bundleenv.Index,
	recon interfaces.EffectiveReconciler,
	reg *registryEnvironmentsBuilder,
) *etcdFollowerWatch {
	return &etcdFollowerWatch{
		config:         cfg,
		etcdClient:     etcd,
		schemaCache:    schemaCache,
		bundleEnvIndex: bundleIndex,
		reconciler:     recon,
		reg:            reg,
	}
}

// start blocks until the watch channel closes (typically ctx cancel). See [APIServerSyncUseCase.StartEtcdFollowerWatch].
//
// etcd Watch does not replay existing keys: without an initial rebuild, followers (and configs with
// environments only in etcd) would serve an empty xDS cache until the next write.
func (f *etcdFollowerWatch) start(ctx context.Context) {
	if f.etcdClient == nil {
		slog.Warn("StartEtcdFollowerWatch: etcd client is nil")
		return
	}

	go func() {
		flushCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		start := time.Now()
		err := f.rebuildAllXDS(flushCtx)
		f.recordRebuildMetrics(ctrlmetrics.RebuildPhaseInitial, err, time.Since(start))
		if err != nil {
			slog.Error("initial xDS rebuild from etcd (HA / cold start)", "error", err)
		} else {
			slog.Info("initial xDS rebuild from etcd completed")
		}
	}()

	ch := f.etcdClient.Watch(ctx, ctrlrepoetcd.ControllerWatchPrefix, clientv3.WithPrefix())

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

		flushCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		start := time.Now()
		var err error
		if full {
			err = f.rebuildAllXDS(flushCtx)
		} else {
			names := make([]string, 0, len(toFlush))
			for e := range toFlush {
				names = append(names, e)
			}
			err = f.rebuildXDSForEnvironments(flushCtx, names)
		}
		f.recordRebuildMetrics(ctrlmetrics.RebuildPhaseDebounced, err, time.Since(start))
		if err != nil {
			slog.Error("etcd follower watch: rebuild xDS", "error", err)
		}
	}

	en := f.config.MetricsHTTP.Enabled
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
				if f.schemaCache != nil {
					f.schemaCache.InvalidateBundleKey(eff.SchemaBundleKey)
				}
				envNames := f.environmentsForBundleKey(ctx, eff.SchemaBundleKey)
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
				if f.bundleEnvIndex != nil {
					rctx, cancel := context.WithTimeout(ctx, 120*time.Second)
					f.bundleEnvIndex.Rebuild(rctx)
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

func (f *etcdFollowerWatch) environmentsForBundleKey(ctx context.Context, bundleKey string) []string {
	if f.bundleEnvIndex == nil {
		return nil
	}
	envs := f.bundleEnvIndex.EnvironmentsForBundle(bundleKey)
	if len(envs) > 0 {
		return envs
	}
	rctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	f.bundleEnvIndex.Rebuild(rctx)
	cancel()
	ctrlmetrics.RecordBundleEnvIndexRebuild(f.config.MetricsHTTP.Enabled)
	return f.bundleEnvIndex.EnvironmentsForBundle(bundleKey)
}

func (f *etcdFollowerWatch) rebuildAllXDS(ctx context.Context) error {
	names, listWarns := f.reg.collectEnvironmentNames(ctx)
	if len(listWarns) > 0 {
		observeNameListDegradationForFollower(ctx, f.config, listWarns)
	}
	return f.rebuildXDSForEnvironments(ctx, names)
}

func (f *etcdFollowerWatch) rebuildXDSForEnvironments(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	sem := semaphore.NewWeighted(xdsRebuildParallelism)
	var wg sync.WaitGroup
	var failMu sync.Mutex
	var nFail int
	var firstErr error
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				return
			}
			defer sem.Release(1)
			if err := f.updateXDSSnapshot(ctx, name); err != nil {
				failMu.Lock()
				nFail++
				if firstErr == nil {
					firstErr = err
				}
				failMu.Unlock()
				slog.Warn("rebuildXDSForEnvironments: environment", "name", name, "error", err)
			}
		}()
	}
	wg.Wait()
	if nFail > 0 {
		if firstErr != nil {
			return fmt.Errorf("rebuildXDSForEnvironments: %d of %d environments failed: %w", nFail, len(names), firstErr)
		}
		return fmt.Errorf("rebuildXDSForEnvironments: %d of %d environments failed", nFail, len(names))
	}
	return nil
}

func (f *etcdFollowerWatch) updateXDSSnapshot(ctx context.Context, environment string) error {
	slog.Info("Updating xDS snapshot", "environment", environment)
	if f.reconciler == nil {
		return nil
	}
	// No materialized writes on follower / hot path (leader CRUD and memory rebuild use writeMaterialized).
	return f.reconciler.ReconcileOne(ctx, environment, false)
}

func (f *etcdFollowerWatch) recordRebuildMetrics(phase string, err error, d time.Duration) {
	en := f.config.MetricsHTTP.Enabled
	res := ctrlmetrics.XDSResultOK
	if err != nil {
		res = ctrlmetrics.XDSResultError
	}
	ctrlmetrics.RecordXDSRebuildFlush(en, phase, res)
	ctrlmetrics.ObserveXDSRebuildDuration(en, phase, d)
}
