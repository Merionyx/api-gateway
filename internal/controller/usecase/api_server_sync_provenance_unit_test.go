package usecase

import (
	"testing"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

func TestStaticConfigToPB_andProvenancePB(t *testing.T) {
	t.Parallel()
	unknown := envmodel.StaticConfigSource(99)
	if staticConfigToPB(unknown) != pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED {
		t.Fatal("bad enum")
	}
	if provenancePB(unknown) != nil {
		t.Fatal()
	}
	if p := provenancePB(envmodel.StaticConfigFile); p == nil || p.ConfigSource != pb.ConfigSource_CONFIG_SOURCE_FILE {
		t.Fatal()
	}
}

func TestProvenanceWithLayer(t *testing.T) {
	t.Parallel()
	unknown := envmodel.StaticConfigSource(99)
	if provenanceWithLayer(unknown, "") != nil {
		t.Fatal()
	}
	if p := provenanceWithLayer(unknown, "x"); p == nil || p.GetLayerDetail() != "x" {
		t.Fatalf("%#v", p)
	}
	if p := provenanceWithLayer(envmodel.StaticConfigFile, "layer"); p == nil || p.GetLayerDetail() != "layer" {
		t.Fatal()
	}
}

func TestEnvironmentDominantLayerDetail(t *testing.T) {
	t.Parallel()
	if s := environmentDominantLayerDetail(false, false, true); s != "dominant:etcd_grpc" {
		t.Fatalf("%q", s)
	}
	if s := environmentDominantLayerDetail(true, true, true); s != "dominant:etcd_grpc" {
		t.Fatalf("etcd first: %q", s)
	}
	if s := environmentDominantLayerDetail(false, true, false); s != "dominant:memory_kubernetes" {
		t.Fatalf("%q", s)
	}
	if s := environmentDominantLayerDetail(true, false, false); s != "dominant:file" {
		t.Fatalf("%q", s)
	}
	if s := environmentDominantLayerDetail(false, false, false); s != "dominant:unspecified" {
		t.Fatalf("%q", s)
	}
}

func TestStaticBundleProvenanceLayer(t *testing.T) {
	t.Parallel()
	if s := staticBundleProvenanceLayer(envmodel.StaticConfigEtcdGRPC, "x"); s != "static:etcd_grpc" {
		t.Fatalf("%q", s)
	}
	if s := staticBundleProvenanceLayer(envmodel.StaticConfigKubernetes, "ns/n"); s != "crd/ContractBundle:ns/n" {
		t.Fatalf("%q", s)
	}
	if s := staticBundleProvenanceLayer(envmodel.StaticConfigKubernetes, ""); s != "static:kubernetes" {
		t.Fatalf("%q", s)
	}
	if s := staticBundleProvenanceLayer(envmodel.StaticConfigUnspecified, ""); s != "" {
		t.Fatalf("%q", s)
	}
}

func TestStaticServiceProvenanceLayer(t *testing.T) {
	t.Parallel()
	if s := staticServiceProvenanceLayer(envmodel.StaticConfigEtcdGRPC, "d"); s != "static:etcd_grpc;discovery:d" {
		t.Fatalf("%q", s)
	}
	if s := staticServiceProvenanceLayer(envmodel.StaticConfigKubernetes, ""); s != "static:kubernetes" {
		t.Fatalf("%q", s)
	}
	if s := staticServiceProvenanceLayer(envmodel.StaticConfigUnspecified, "only"); s != "discovery:only" {
		t.Fatalf("%q", s)
	}
}

func TestEtcdStaticExtractors(t *testing.T) {
	t.Parallel()
	if etcdStaticBundles(nil) != nil {
		t.Fatal()
	}
	e := &models.Environment{Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b"}}}}
	if b := etcdStaticBundles(e); len(b) != 1 {
		t.Fatal()
	}
	if etcdStaticServices(nil) != nil {
		t.Fatal()
	}
	es := &models.Environment{Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s"}}}}
	if s := etcdStaticServices(es); len(s) != 1 {
		t.Fatal()
	}
}

func TestSharedToControllerSnapshot_minimal(t *testing.T) {
	t.Parallel()
	out := sharedToControllerSnapshot(sharedgit.ContractSnapshot{
		Name:     "n",
		Prefix:   "/p",
		Upstream: sharedgit.ContractUpstream{Name: "u"},
		Access:   sharedgit.Access{Secure: true, Apps: []sharedgit.App{{AppID: "a"}}},
	})
	if out == nil || out.Name != "n" {
		t.Fatalf("%#v", out)
	}
}
