package etcd

import "testing"

func TestEnvironmentKey(t *testing.T) {
	const name = "prod-eu"
	got := EnvironmentPrefix + name + "/config"
	want := "/api-gateway/controller/environments/prod-eu/config"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEffectiveMaterializedLayout(t *testing.T) {
	if EffectiveMaterializedPrefix+`staging/v1` != "/api-gateway/controller/effective/staging/v1" {
		t.Fatalf("effective prefix layout: %q", EffectiveMaterializedPrefix)
	}
}
