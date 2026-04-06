package cache

import (
	"testing"

	"merionyx/api-gateway/internal/controller/repository/etcd"
)

func TestParseEnvironmentNameFromConfigKey(t *testing.T) {
	key := etcd.EnvironmentPrefix + "staging/config"
	name, ok := ParseEnvironmentNameFromConfigKey(key)
	if !ok || name != "staging" {
		t.Fatalf("got %q ok=%v", name, ok)
	}
}

func TestParseEnvironmentNameFromConfigKey_invalid(t *testing.T) {
	cases := []string{
		"",
		// Missing env segment: suffix still matches, but TrimPrefix leaves a path with slash.
		etcd.EnvironmentPrefix + "/config",
		etcd.EnvironmentPrefix + "a/b/config",
		etcd.EnvironmentPrefix + "//config",
		"/api-gateway/other/environments/x/config",
	}
	for _, k := range cases {
		if n, ok := ParseEnvironmentNameFromConfigKey(k); ok {
			t.Errorf("%q: unexpected ok, name=%q", k, n)
		}
	}
}
