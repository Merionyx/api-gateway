package idempotency

import (
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestIdempotencyKeyFingerprint(t *testing.T) {
	t.Parallel()
	a := idempotencyKeyFingerprint("same-key")
	b := idempotencyKeyFingerprint("same-key")
	if a != b || len(a) != 64 {
		t.Fatalf("deterministic fingerprint: %q %q", a, b)
	}
	if idempotencyKeyFingerprint("other") == a {
		t.Fatal("different keys must differ")
	}
}

func TestNewEtcdStore_prefixAndKey(t *testing.T) {
	t.Parallel()
	// Client required; use nil is invalid — only verify prefix/key helpers via a dummy client is impossible.
	// We still validate key shape using a fake client pointer only for struct field assignment:
	cl := &clientv3.Client{}
	e := NewEtcdStore(cl, "  /custom/prefix/  ", time.Minute)
	if e.prefix != "/custom/prefix/keys/" {
		t.Fatalf("prefix %q", e.prefix)
	}
	fp := idempotencyKeyFingerprint("my-key")
	wantKey := e.prefix + fp
	if got := e.etcdKey("my-key"); got != wantKey {
		t.Fatalf("etcdKey %q want %q", got, wantKey)
	}
}
