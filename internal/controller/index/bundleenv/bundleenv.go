// Package bundleenv maintains bundle key → environment names for targeted rebuilds and notifications.
package bundleenv

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// Index maps bundle keys (repository/escapedRef/escapedPath) to environment names that reference the bundle.
type Index struct {
	envEtcd interfaces.EnvironmentRepository
	envMem  interfaces.InMemoryEnvironmentsRepository
	mu      sync.RWMutex
	// bundleKey -> sorted unique environment names
	bundleTo map[string][]string
}

func NewIndex(envEtcd interfaces.EnvironmentRepository, envMem interfaces.InMemoryEnvironmentsRepository) *Index {
	return &Index{
		envEtcd:  envEtcd,
		envMem:   envMem,
		bundleTo: make(map[string][]string),
	}
}

func (i *Index) getEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	if i.envEtcd != nil {
		if e, err := i.envEtcd.GetEnvironment(ctx, name); err == nil {
			return e, nil
		}
	}
	if i.envMem != nil {
		return i.envMem.GetEnvironment(ctx, name)
	}
	return nil, errors.New("no environment repository")
}

// Rebuild scans all environments from etcd and in-memory sources and rebuilds the index.
func (i *Index) Rebuild(ctx context.Context) {
	names := make(map[string]struct{})
	if i.envEtcd != nil {
		if m, err := i.envEtcd.ListEnvironments(ctx); err == nil {
			for k := range m {
				names[k] = struct{}{}
			}
		}
	}
	if i.envMem != nil {
		if m, err := i.envMem.ListEnvironments(ctx); err == nil {
			for k := range m {
				names[k] = struct{}{}
			}
		}
	}

	next := make(map[string]map[string]struct{})
	for envName := range names {
		env, err := i.getEnvironment(ctx, envName)
		if err != nil || env == nil || env.Bundles == nil {
			continue
		}
		for _, b := range env.Bundles.Static {
			bk := bundlekey.Build(b.Repository, b.Ref, b.Path)
			if next[bk] == nil {
				next[bk] = make(map[string]struct{})
			}
			next[bk][envName] = struct{}{}
		}
	}

	flat := make(map[string][]string, len(next))
	for bk, set := range next {
		list := make([]string, 0, len(set))
		for e := range set {
			list = append(list, e)
		}
		sort.Strings(list)
		flat[bk] = list
	}

	i.mu.Lock()
	i.bundleTo = flat
	i.mu.Unlock()
}

// EnvironmentsForBundle returns environment names that use the bundle (copy).
func (i *Index) EnvironmentsForBundle(bundleKey string) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	v := i.bundleTo[bundleKey]
	if len(v) == 0 {
		return nil
	}
	out := make([]string, len(v))
	copy(out, v)
	return out
}
