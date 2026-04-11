package registry

import (
	"context"
	"testing"
)

func TestStatusReadUseCase_CheckEtcd_nilClient(t *testing.T) {
	t.Parallel()
	u := NewStatusReadUseCase(nil, nil)
	if got := u.CheckEtcd(context.Background()); got != "error" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusReadUseCase_Readiness_nilEtcd(t *testing.T) {
	t.Parallel()
	u := NewStatusReadUseCase(nil, nil)
	r := u.Readiness(context.Background(), false)
	if r.Status != "not_ready" || r.Etcd != "error" || r.ContractSyncer != "skipped" {
		t.Fatalf("unexpected: %+v", r)
	}
}
