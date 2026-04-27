package etcd

import "testing"

func TestTokenGrantRepository_tokenGrantKey(t *testing.T) {
	t.Parallel()
	r := NewTokenGrantRepository(nil, "/api-gateway/api-server/auth/v1")
	jti := "550e8400-e29b-41d4-a716-446655440000"
	key, err := r.tokenGrantKey(jti)
	if err != nil {
		t.Fatal(err)
	}
	want := "/api-gateway/api-server/auth/v1/token-grants/" + jti
	if key != want {
		t.Fatalf("key %q want %q", key, want)
	}
}

func TestValidateTokenJTI(t *testing.T) {
	t.Parallel()
	if _, err := validateTokenJTI("550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, bad := range []string{"", "not-a-uuid", "550E8400-E29B-41D4-A716-446655440000"} {
		if _, err := validateTokenJTI(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
