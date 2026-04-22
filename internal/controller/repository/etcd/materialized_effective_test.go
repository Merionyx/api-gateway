package etcd

import (
	"context"
	"testing"
)

func TestMaterializedStore_Delete_nil(t *testing.T) {
	var s *MaterializedStore
	if err := s.Delete(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializedStore_Delete_emptyName(t *testing.T) {
	s := NewMaterializedStore(nil) // also safe
	if err := s.Delete(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}
