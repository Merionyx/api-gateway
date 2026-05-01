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

func TestCanonicalEnvironmentsForStorage_metaCopy(t *testing.T) {
	t.Parallel()
	gen := int64(7)
	sv := int32(2)
	mm := true
	in := []EnvironmentInfo{{
		Name: "a",
		Bundles: []BundleInfo{{
			Name: "b", Repository: "r", Ref: "f", Path: "p",
			Meta: &BundleMeta{Provenance: &Provenance{ConfigSource: "file"}, ResolvedRef: "r@h"},
		}},
		Services: []ServiceInfo{{
			Name: "s", Upstream: "u",
			Meta: &ServiceMeta{Provenance: &Provenance{ConfigSource: "k8s"}, K8sServiceRef: "x"},
		}},
		Meta: &EnvironmentMeta{
			Provenance:                &Provenance{ConfigSource: "controller"},
			EffectiveGeneration:       &gen,
			SourcesFingerprint:        "fp",
			EnvironmentType:           "t",
			MaterializedUpdatedAt:     "u",
			MaterializedSchemaVersion: &sv,
			MaterializedMismatch:      &mm,
		},
	}}
	got := CanonicalEnvironmentsForStorage(in)
	if len(got) != 1 {
		t.Fatal()
	}
	if got[0].Bundles[0].Meta == in[0].Bundles[0].Meta {
		t.Fatal("bundle meta should be deep-copied")
	}
	if got[0].Services[0].Meta == in[0].Services[0].Meta {
		t.Fatal("service meta should be deep-copied")
	}
}

func TestEnvironmentMetaConfigSource(t *testing.T) {
	t.Parallel()
	if EnvironmentMetaConfigSource(nil) != "" {
		t.Fatal()
	}
	if EnvironmentMetaConfigSource(&EnvironmentMeta{}) != "" {
		t.Fatal()
	}
	if s := EnvironmentMetaConfigSource(&EnvironmentMeta{Provenance: &Provenance{ConfigSource: "c"}}); s != "c" {
		t.Fatalf("got %q", s)
	}
}
