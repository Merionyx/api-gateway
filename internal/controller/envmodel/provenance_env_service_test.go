package envmodel

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestConfigSourceForEnvironmentLayers(t *testing.T) {
	if g := ConfigSourceForEnvironmentLayers(true, true, true); g != StaticConfigEtcdGRPC {
		t.Fatalf("expected etcd when all set: %v", g)
	}
	if g := ConfigSourceForEnvironmentLayers(true, true, false); g != StaticConfigKubernetes {
		t.Fatalf("expected k8s: %v", g)
	}
	if g := ConfigSourceForEnvironmentLayers(true, false, false); g != StaticConfigFile {
		t.Fatalf("expected file: %v", g)
	}
}

func TestConfigSourceForStaticService(t *testing.T) {
	s := models.StaticServiceConfig{Name: "a", Upstream: "u"}
	etcd := []models.StaticServiceConfig{{Name: "a", Upstream: "x"}}
	if g := ConfigSourceForStaticService(s, nil, nil, etcd); g != StaticConfigEtcdGRPC {
		t.Fatal(g)
	}
}
