package envmodel

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestConfigSourceForStaticBundle_etcdWins(t *testing.T) {
	b := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	file := []models.StaticContractBundleConfig{b}
	k8s := []models.StaticContractBundleConfig{{Name: "b", Repository: "r", Ref: "2", Path: "p"}}
	etcd := []models.StaticContractBundleConfig{b}
	eff := b
	if g := ConfigSourceForStaticBundle(eff, file, k8s, etcd); g != StaticConfigEtcdGRPC {
		t.Fatalf("got %v", g)
	}
}

func TestConfigSourceForStaticBundle_k8sOverFile(t *testing.T) {
	bk := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "2", Path: "p"}
	bf := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	if g := ConfigSourceForStaticBundle(bk, []models.StaticContractBundleConfig{bf}, []models.StaticContractBundleConfig{bk}, nil); g != StaticConfigKubernetes {
		t.Fatalf("got %v", g)
	}
}

func TestConfigSourceForStaticBundle_fileOnly(t *testing.T) {
	b := models.StaticContractBundleConfig{Name: "b", Repository: "r", Ref: "1", Path: "p"}
	if g := ConfigSourceForStaticBundle(b, []models.StaticContractBundleConfig{b}, nil, nil); g != StaticConfigFile {
		t.Fatalf("got %v", g)
	}
}
