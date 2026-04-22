package idempotency

import "testing"

func TestResolveKeyPrefix(t *testing.T) {
	t.Parallel()
	if got := ResolveKeyPrefix("", ""); got != "/api-gateway/api-server/idempotency/v1" {
		t.Fatalf("empty base: %q", got)
	}
	if got := ResolveKeyPrefix("/custom", ""); got != "/custom" {
		t.Fatalf("custom base: %q", got)
	}
	if got := ResolveKeyPrefix("/custom", "prod"); got != "/custom/clusters/prod" {
		t.Fatalf("with cluster: %q", got)
	}
	if got := ResolveKeyPrefix("/custom", "a/b"); got != "/custom/clusters/a_b" {
		t.Fatalf("sanitized cluster: %q", got)
	}
	if got := ResolveKeyPrefix("  /x/  ", "c"); got != "/x/clusters/c" {
		t.Fatalf("trim: %q", got)
	}
}
