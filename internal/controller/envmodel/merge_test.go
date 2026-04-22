package envmodel

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestBundleKey_dedupes(t *testing.T) {
	b1 := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	b2 := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	if BundleKey(b1) != BundleKey(b2) {
		t.Fatalf("expected same key")
	}
}

func TestUnionStaticBundles(t *testing.T) {
	a := []models.StaticContractBundleConfig{
		{Name: "b1", Repository: "r1", Ref: "main", Path: "p1"},
	}
	b := []models.StaticContractBundleConfig{
		{Name: "b2", Repository: "r2", Ref: "main", Path: "p2"},
	}
	u := UnionStaticBundles(a, b)
	if len(u) != 2 {
		t.Fatalf("got %d bundles, want 2: %+v", len(u), u)
	}
}

// Same key from two sources collapses to one; second slice wins (replaces map entry after first).
func TestUnionStaticBundles_sameKey_dedupesToOne(t *testing.T) {
	keyed := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	a := []models.StaticContractBundleConfig{keyed}
	b := []models.StaticContractBundleConfig{keyed}
	u := UnionStaticBundles(a, b)
	if len(u) != 1 {
		t.Fatalf("got %d want 1: %+v", len(u), u)
	}
}

func TestUnionStaticServices_LaterOverrides(t *testing.T) {
	a := []models.StaticServiceConfig{{Name: "s", Upstream: "http://old:1"}}
	b := []models.StaticServiceConfig{{Name: "s", Upstream: "http://new:2"}}
	u := UnionStaticServices(a, b)
	if len(u) != 1 || u[0].Upstream != "http://new:2" {
		t.Fatalf("%+v", u)
	}
}

func TestMergeFileAndK8s_fileAndK8s_mergedNameFromK8s(t *testing.T) {
	file := &models.Environment{
		Name: "e1",
		Type: "static",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "bf", Repository: "rf", Ref: "1", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "sf", Upstream: "http://f:1"}},
		},
	}
	k8s := &models.Environment{
		Name: "e1",
		Type: "kubernetes",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "bk", Repository: "rk", Ref: "2", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "sk", Upstream: "http://k:2"}},
		},
	}
	m := MergeFileAndK8s(file, k8s)
	if m.Name != "e1" || m.Type != "kubernetes" || m.Snapshots != nil {
		t.Fatalf("name/type: %+v", m)
	}
	if len(m.Bundles.Static) != 2 || len(m.Services.Static) != 2 {
		t.Fatalf("bundles=%d services=%d: %+v", len(m.Bundles.Static), len(m.Services.Static), m)
	}
}

func TestMergeFileAndK8s_fileOnly(t *testing.T) {
	file := &models.Environment{
		Name: "e",
		Type: "static",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "b1", Repository: "r", Ref: "1", Path: "p"}},
		},
		Services:  &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s", Upstream: "u"}}},
		Snapshots: []models.ContractSnapshot{{Name: "x"}},
	}
	m := MergeFileAndK8s(file, nil)
	if m.Name != "e" || m.Type != "static" {
		t.Fatalf("got %+v", m)
	}
	if m.Snapshots != nil {
		t.Fatalf("snapshots should be nil, got %v", m.Snapshots)
	}
	if len(m.Bundles.Static) != 1 {
		t.Fatalf("bundles %+v", m.Bundles)
	}
}

func TestMergeFileAndK8s_k8sOnly(t *testing.T) {
	k8s := &models.Environment{
		Name: "e",
		Type: "kubernetes",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "b1", Repository: "r", Ref: "1", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s", Upstream: "u"}}},
	}
	m := MergeFileAndK8s(nil, k8s)
	if m.Name != "e" || m.Type != "kubernetes" {
		t.Fatalf("got %+v", m)
	}
	if m.Snapshots != nil {
		t.Fatalf("snapshots %+v", m.Snapshots)
	}
}

func TestMergeInMemoryWithEtcd_threeSourcesLogical_memThenEtcdUnion(t *testing.T) {
	mem := &models.Environment{
		Name: "env",
		Type: "static",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "b-mem", Repository: "a", Ref: "1", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s-mem", Upstream: "m"}}},
	}
	etcd := &models.Environment{
		Name: "ignored",
		Type: "ignored",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "b-etcd", Repository: "b", Ref: "1", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s-etcd", Upstream: "e"}}},
	}
	m := MergeInMemoryWithEtcd(mem, etcd)
	if m.Name != "env" || m.Type != "static" {
		t.Fatalf("got %+v", m)
	}
	if len(m.Bundles.Static) != 2 || len(m.Services.Static) != 2 {
		t.Fatalf("bundles=%d services=%d", len(m.Bundles.Static), len(m.Services.Static))
	}
}

func TestMergeInMemoryWithEtcd_etcdWinsOnSameServiceName(t *testing.T) {
	mem := &models.Environment{
		Name: "e",
		Type: "t",
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "s", Upstream: "http://mem"}},
		},
		Bundles: &models.EnvironmentBundleConfig{Static: nil},
	}
	etcd := &models.Environment{
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "s", Upstream: "http://etcd"}},
		},
	}
	m := MergeInMemoryWithEtcd(mem, etcd)
	if len(m.Services.Static) != 1 || m.Services.Static[0].Upstream != "http://etcd" {
		t.Fatalf("%+v", m.Services.Static)
	}
}

// Different bundle keys (e.g. different ref) are both kept.
func TestMergeInMemoryWithEtcd_differentBundleKeys_both(t *testing.T) {
	mem := &models.Environment{
		Name:     "e",
		Type:     "t",
		Services: &models.EnvironmentServiceConfig{Static: nil},
		Bundles:  &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b", Repository: "r", Ref: "0", Path: "p"}}},
	}
	etcd := &models.Environment{
		Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b", Repository: "r", Ref: "1", Path: "p"}}},
	}
	m := MergeInMemoryWithEtcd(mem, etcd)
	if len(m.Bundles.Static) != 2 {
		t.Fatalf("want 2 different keys, got %d: %+v", len(m.Bundles.Static), m.Bundles.Static)
	}
}

// Same bundle key from mem and etcd: single entry (etcd is merged second).
func TestMergeInMemoryWithEtcd_sameBundleKey_dedupesToOne(t *testing.T) {
	same := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	mem := &models.Environment{
		Name: "e", Type: "t", Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{same}},
		Services: &models.EnvironmentServiceConfig{Static: nil},
	}
	etcd := &models.Environment{Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{same}}}
	m2 := MergeInMemoryWithEtcd(mem, etcd)
	if len(m2.Bundles.Static) != 1 {
		t.Fatalf("expected one bundle, got %d: %+v", len(m2.Bundles.Static), m2.Bundles.Static)
	}
}

func TestToAPIServerSkeleton_copiesAndClearsSnapshots(t *testing.T) {
	src := &models.Environment{
		Name:     "a",
		Type:     "t",
		Bundles:  &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b", Repository: "r", Ref: "1", Path: "p"}}},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s", Upstream: "u"}}},
		Snapshots: []models.ContractSnapshot{{Name: "keep"}},
	}
	s := ToAPIServerSkeleton(src)
	if s == nil {
		t.Fatal("nil")
	}
	if s.Snapshots != nil {
		t.Fatalf("snapshots %v", s.Snapshots)
	}
	if len(s.Bundles.Static) != 1 || s.Bundles.Static[0].Name != "b" {
		t.Fatalf("bundles %+v", s.Bundles)
	}
	// must not share slice backing with src
	s.Bundles.Static[0].Name = "mut"
	if src.Bundles.Static[0].Name != "b" {
		t.Fatalf("copy was not independent")
	}
}

func TestToAPIServerSkeleton_nil(t *testing.T) {
	if ToAPIServerSkeleton(nil) != nil {
		t.Fatal("expected nil")
	}
}
