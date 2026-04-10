package usecase

import (
	"context"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// runParallelForEachBundle runs fn for each bundle with at most maxParallel concurrent calls.
func runParallelForEachBundle(ctx context.Context, bundles []models.BundleInfo, maxParallel int64, fn func(context.Context, models.BundleInfo)) {
	if len(bundles) == 0 {
		return
	}
	sem := semaphore.NewWeighted(maxParallel)
	g, gctx := errgroup.WithContext(ctx)
	for i := range bundles {
		b := bundles[i]
		g.Go(func() error {
			if err := sem.Acquire(gctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)
			fn(gctx, b)
			return nil
		})
	}
	_ = g.Wait()
}
