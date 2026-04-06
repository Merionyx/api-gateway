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
