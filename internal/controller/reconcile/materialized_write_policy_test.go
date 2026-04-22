package reconcile

import (
	"context"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/election"
)

type testLeader struct{ isLeader bool }

func (l *testLeader) IsLeader() bool { return l.isLeader }

func (l *testLeader) LeaderChanged() <-chan struct{} { return nil }

var _ election.LeaderGate = (*testLeader)(nil)

type testMatStore struct{}

func (testMatStore) ReconcileIfChanged(context.Context, *models.Environment) error { return nil }

func (testMatStore) Delete(context.Context, string) error { return nil }

func TestMaterializedWritePolicy_Allow(t *testing.T) {
	t.Parallel()
	s := &testMatStore{}
	ln := &testLeader{isLeader: true}
	lf := &testLeader{isLeader: false}
	var nilSt *testMatStore

	if !NewMaterializedWritePolicy(true, s, ln).Allow() {
		t.Error("enabled + store + leader: want allow")
	}
	if NewMaterializedWritePolicy(false, s, ln).Allow() {
		t.Error("disabled: want no allow")
	}
	if NewMaterializedWritePolicy(true, nilSt, ln).Allow() {
		t.Error("nil store: want no allow")
	}
	if NewMaterializedWritePolicy(true, s, nil).Allow() {
		t.Error("nil leader gate: want no allow")
	}
	if NewMaterializedWritePolicy(true, s, lf).Allow() {
		t.Error("not leader: want no allow")
	}
}
