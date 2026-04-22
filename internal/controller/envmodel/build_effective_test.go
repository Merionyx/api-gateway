package envmodel

import (
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestBuildOptionalEffectiveEnvironment_bothNil(t *testing.T) {
	_, err := BuildOptionalEffectiveEnvironment(nil, nil)
	if !errors.Is(err, ErrBuildEffectiveNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestBuildOptionalEffectiveEnvironment_etcdOnly(t *testing.T) {
	etcd := &models.Environment{
		Name: "e", Type: "t",
		Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b", Repository: "r", Ref: "1", Path: "p"}}},
	}
	e, err := BuildOptionalEffectiveEnvironment(nil, etcd)
	if err != nil || e.Name != "e" {
		t.Fatalf("got %+v %v", e, err)
	}
}
