package bundle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

func TestRunParallelForEachBundle_respectsMaxParallel(t *testing.T) {
	const n = 24
	const maxP = 5
	bundles := make([]models.BundleInfo, n)
	for i := range bundles {
		bundles[i] = models.BundleInfo{Name: "b", Repository: "r", Ref: "main", Path: "p"}
	}

	var cur atomic.Int32
	var peak atomic.Int32

	ctx := context.Background()
	runParallelForEachBundle(ctx, bundles, maxP, func(_ context.Context, _ models.BundleInfo) {
		v := cur.Add(1)
		for {
			old := peak.Load()
			if int32(v) <= old {
				break
			}
			if peak.CompareAndSwap(old, int32(v)) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		cur.Add(-1)
	})

	if int(peak.Load()) > maxP {
		t.Fatalf("peak concurrency %d > max %d", peak.Load(), maxP)
	}
	if peak.Load() < 1 {
		t.Fatalf("expected some parallelism")
	}
}
