package cache

import (
	"context"
	"sync"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// SchemaCache wraps SchemaRepository and caches ListContractSnapshots per bundle key.
type SchemaCache struct {
	inner          interfaces.SchemaRepository
	metricsEnabled bool
	mu             sync.RWMutex
	// bundleKey -> snapshots (owned copy)
	list map[string][]models.ContractSnapshot
}

func NewSchemaCache(inner interfaces.SchemaRepository, metricsEnabled bool) *SchemaCache {
	return &SchemaCache{
		inner:          inner,
		metricsEnabled: metricsEnabled,
		list:           make(map[string][]models.ContractSnapshot),
	}
}

// Inner returns the wrapped repository (e.g. for tests).
func (c *SchemaCache) Inner() interfaces.SchemaRepository {
	return c.inner
}

// InvalidateBundle drops cached list for one bundle.
func (c *SchemaCache) InvalidateBundle(repository, ref, bundlePath string) {
	k := bundlekey.Build(repository, ref, bundlePath)
	c.mu.Lock()
	delete(c.list, k)
	c.mu.Unlock()
}

// InvalidateBundleKey drops cache entry by bundle key from Build().
func (c *SchemaCache) InvalidateBundleKey(bundleKey string) {
	c.mu.Lock()
	delete(c.list, bundleKey)
	c.mu.Unlock()
}

func (c *SchemaCache) SaveContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string, snapshot *models.ContractSnapshot) error {
	err := c.inner.SaveContractSnapshot(ctx, repo, ref, bundlePath, contract, snapshot)
	if err == nil {
		c.InvalidateBundle(repo, ref, bundlePath)
	}
	return err
}

func (c *SchemaCache) GetContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string) (*models.ContractSnapshot, error) {
	return c.inner.GetContractSnapshot(ctx, repo, ref, bundlePath, contract)
}

func (c *SchemaCache) GetEnvironmentSnapshots(ctx context.Context, envName string) ([]models.ContractSnapshot, error) {
	return c.inner.GetEnvironmentSnapshots(ctx, envName)
}

func (c *SchemaCache) ListContractSnapshots(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error) {
	k := bundlekey.Build(repository, ref, bundlePath)
	c.mu.RLock()
	if v, ok := c.list[k]; ok {
		c.mu.RUnlock()
		ctrlmetrics.RecordSchemaListCacheHit(c.metricsEnabled, true)
		return cloneContractSnapshots(v), nil
	}
	c.mu.RUnlock()

	ctrlmetrics.RecordSchemaListCacheHit(c.metricsEnabled, false)
	snaps, err := c.inner.ListContractSnapshots(ctx, repository, ref, bundlePath)
	if err != nil {
		return nil, err
	}
	cp := cloneContractSnapshots(snaps)
	c.mu.Lock()
	c.list[k] = cloneContractSnapshots(snaps)
	c.mu.Unlock()
	return cp, nil
}

func (c *SchemaCache) WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan {
	return c.inner.WatchContractBundlesSnapshots(ctx)
}

func cloneContractSnapshots(in []models.ContractSnapshot) []models.ContractSnapshot {
	if len(in) == 0 {
		return nil
	}
	out := make([]models.ContractSnapshot, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Access.Apps = append([]models.App(nil), in[i].Access.Apps...)
		for j := range out[i].Access.Apps {
			out[i].Access.Apps[j].Environments = append([]string(nil), in[i].Access.Apps[j].Environments...)
		}
	}
	return out
}
