package memory

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestServiceRepository_Initialize_Get_List(t *testing.T) {
	r := NewServiceRepository()
	cfg := &config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{
				{Name: "a", Upstream: "http://a:1"},
				{Name: "b", Upstream: "http://b:2"},
			},
		},
	}
	if err := r.Initialize(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetService("a")
	if err != nil || got.Name != "a" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := r.GetService("missing"); err == nil {
		t.Fatal("expected error")
	}
	r.SetKubernetesGlobalServices([]models.StaticServiceConfig{{Name: "k", Upstream: "http://k:3"}})
	ks, err := r.GetService("k")
	if err != nil || ks.Name != "k" {
		t.Fatalf("%+v %v", ks, err)
	}
	list, err := r.ListServices()
	if err != nil || len(list) < 3 {
		t.Fatalf("list len %d err %v", len(list), err)
	}
	filePool, kubePool := r.ListRootPoolDeduplicated()
	if len(filePool) != 2 || len(kubePool) != 1 || kubePool[0].Name != "k" {
		t.Fatalf("dedup: file=%v kube=%v", filePool, kubePool)
	}
}

func TestServiceRepository_ListRootPoolDeduplicated_staticWinsOverKube(t *testing.T) {
	r := NewServiceRepository()
	_ = r.Initialize(&config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{
				{Name: "dup", Upstream: "http://file:1"},
			},
		},
	})
	r.SetKubernetesGlobalServices([]models.StaticServiceConfig{
		{Name: "dup", Upstream: "http://kube:2"},
		{Name: "only-k", Upstream: "http://k:3"},
	})
	f, k := r.ListRootPoolDeduplicated()
	if len(f) != 1 || f[0].Upstream != "http://file:1" {
		t.Fatalf("file: %+v", f)
	}
	if len(k) != 1 || k[0].Name != "only-k" {
		t.Fatalf("kube: %+v", k)
	}
}
