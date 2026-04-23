package etcd

import (
	"strings"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
)

func TestValidateCanonicalSessionUUIDv4(t *testing.T) {
	t.Parallel()
	valid := "550e8400-e29b-41d4-a716-446655440000"
	got, err := validateCanonicalSessionUUIDv4(valid)
	if err != nil || got != valid {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, bad := range []string{
		"",
		"not-a-uuid",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8", // UUID v1
		"550E8400-E29B-41D4-A716-446655440000", // wrong casing
	} {
		_, err := validateCanonicalSessionUUIDv4(bad)
		if err == nil {
			t.Fatalf("want error for %q", bad)
		}
	}
}

func TestSessionRepository_sessionKey(t *testing.T) {
	t.Parallel()
	r := NewSessionRepository(nil, "/api-gateway/api-server/auth/v1")
	id := "550e8400-e29b-41d4-a716-446655440000"
	key, err := r.sessionKey(id)
	if err != nil {
		t.Fatal(err)
	}
	want := "/api-gateway/api-server/auth/v1/sessions/" + id
	if key != want {
		t.Fatalf("key %q want %q", key, want)
	}
}

func TestReplaceCAS_InvalidRevision(t *testing.T) {
	t.Parallel()
	r := NewSessionRepository(nil, DefaultAuthEtcdKeyPrefix)
	err := r.ReplaceCAS(t.Context(), "550e8400-e29b-41d4-a716-446655440000", kvvalue.SessionValue{
		EncryptedIDPRefresh: []byte(`{}`),
	}, 0)
	if err == nil || !strings.Contains(err.Error(), "expected_mod_revision") {
		t.Fatalf("got %v", err)
	}
}
