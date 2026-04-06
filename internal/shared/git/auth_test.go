package git

import (
	"strings"
	"testing"
)

func TestResolveTransportAuth_NoneAndEmpty(t *testing.T) {
	for _, typ := range []string{"none", ""} {
		a, err := resolveTransportAuth(AuthConfig{Type: typ}, "r")
		if err != nil {
			t.Fatalf("%q: %v", typ, err)
		}
		if a != nil {
			t.Fatalf("%q: expected nil auth", typ)
		}
	}
}

func TestResolveTransportAuth_Unsupported(t *testing.T) {
	_, err := resolveTransportAuth(AuthConfig{Type: "ldap"}, "repo")
	if err == nil || !strings.Contains(err.Error(), "unsupported auth type") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveTransportAuth_TokenMissingEnvVar(t *testing.T) {
	_, err := resolveTransportAuth(AuthConfig{Type: "token"}, "repo")
	if err == nil || !strings.Contains(err.Error(), "no token environment variable") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveTransportAuth_TokenEmptyEnv(t *testing.T) {
	t.Setenv("GIT_TEST_EMPTY_TOKEN", "")
	_, err := resolveTransportAuth(AuthConfig{Type: "token", TokenEnv: "GIT_TEST_EMPTY_TOKEN"}, "repo")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveTransportAuth_SSHNoKey(t *testing.T) {
	_, err := resolveTransportAuth(AuthConfig{Type: "ssh"}, "repo")
	if err == nil || !strings.Contains(err.Error(), "no SSH key") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveTransportAuth_SSHBadKeyFile(t *testing.T) {
	_, err := resolveTransportAuth(AuthConfig{Type: "ssh", SSHKeyPath: "/nonexistent/key.pem"}, "repo")
	if err == nil || !strings.Contains(err.Error(), "failed to load SSH key") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveTransportAuth_TokenOK(t *testing.T) {
	t.Setenv("GIT_TEST_OK_TOKEN", "tok")
	a, err := resolveTransportAuth(AuthConfig{Type: "token", TokenEnv: "GIT_TEST_OK_TOKEN"}, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected auth")
	}
}

func TestResolveTransportAuth_SSHKeyEnvEmptyPath(t *testing.T) {
	t.Setenv("GIT_TEST_EMPTY_SSH", "")
	_, err := resolveTransportAuth(AuthConfig{Type: "ssh", SSHKeyEnv: "GIT_TEST_EMPTY_SSH"}, "repo")
	if err == nil || !strings.Contains(err.Error(), "failed to load SSH key") {
		t.Fatalf("got %v", err)
	}
}
