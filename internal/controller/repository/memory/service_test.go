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
	k, err := r.GetService("k")
	if err != nil || k.Name != "k" {
		t.Fatalf("%+v %v", k, err)
	}
	list, err := r.ListServices()
	if err != nil || len(list) < 3 {
		t.Fatalf("list len %d err %v", len(list), err)
	}
}
