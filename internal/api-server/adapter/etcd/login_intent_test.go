package etcd

import (
	"testing"
)

func TestLoginIntentRepository_intentKey(t *testing.T) {
	t.Parallel()
	r := NewLoginIntentRepository(nil, "/api-gateway/api-server/auth/v1")
	id := "6ba7b810-9dad-41d4-a716-446655440000"
	key, err := r.intentKey(id)
	if err != nil {
		t.Fatal(err)
	}
	want := "/api-gateway/api-server/auth/v1/login-intents/" + id
	if key != want {
		t.Fatalf("key %q want %q", key, want)
	}
}
