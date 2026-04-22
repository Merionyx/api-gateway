package reconcile

import (
	"context"
	"testing"
)

func TestReconciler_RebuildAllFromMemory_noXDS(t *testing.T) {
	r := New(ReconcilerDeps{})
	// nil snapshot manager: no-op, must not panic
	r.RebuildAllFromMemory(context.Background(), nil)
}
