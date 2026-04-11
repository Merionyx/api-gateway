package models

import (
	"reflect"
	"testing"
)

func TestCanonicalEnvironmentsForStorage_nilOrEmpty(t *testing.T) {
	t.Parallel()
	if got := CanonicalEnvironmentsForStorage(nil); got != nil {
		t.Fatalf("nil in: want nil out, got %#v", got)
	}
	if got := CanonicalEnvironmentsForStorage([]EnvironmentInfo{}); got != nil {
		t.Fatalf("empty in: want nil out, got %#v", got)
	}
}

func TestCanonicalEnvironmentsForStorage_sorting(t *testing.T) {
	t.Parallel()
	in := []EnvironmentInfo{
		{
			Name: "prod",
			Bundles: []BundleInfo{
				{Name: "b2", Repository: "r", Ref: "main", Path: "p2"},
				{Name: "b1", Repository: "r", Ref: "main", Path: "p1"},
			},
		},
		{Name: "dev", Bundles: []BundleInfo{{Name: "only", Repository: "x", Ref: "y", Path: "z"}}},
	}
	got := CanonicalEnvironmentsForStorage(in)
	want := []EnvironmentInfo{
		{
			Name: "dev",
			Bundles: []BundleInfo{
				{Name: "only", Repository: "x", Ref: "y", Path: "z"},
			},
		},
		{
			Name: "prod",
			Bundles: []BundleInfo{
				{Name: "b1", Repository: "r", Ref: "main", Path: "p1"},
				{Name: "b2", Repository: "r", Ref: "main", Path: "p2"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
