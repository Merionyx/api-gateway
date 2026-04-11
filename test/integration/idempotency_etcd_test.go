//go:build integration

package integration

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/idempotency"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdIdempotency_Replay502AndConflict(t *testing.T) {
	cli := NewEtcdClient(t)
	defer func() { _ = cli.Close() }()

	pfx := "/test/api-server-idempotency/" + t.Name()
	defer deletePrefix(t, cli, pfx)

	store := idempotency.NewEtcdStore(cli, pfx, 10*time.Minute)
	ctx := context.Background()
	ik := "0123456789abcdef0123456789abcdef"
	h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	var runs int32
	res1, err := store.Execute(ctx, ik, h1, func() (*idempotency.HTTPResult, error) {
		atomic.AddInt32(&runs, 1)
		return &idempotency.HTTPResult{
			StatusCode:  502,
			ContentType: "application/problem+json",
			Body:        []byte(`{"title":"Bad Gateway","status":502}`),
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res1.StatusCode != 502 {
		t.Fatalf("first status: %d", res1.StatusCode)
	}

	res2, err := store.Execute(ctx, ik, h1, func() (*idempotency.HTTPResult, error) {
		t.Fatal("replay must not invoke fn")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2.StatusCode != res1.StatusCode || string(res2.Body) != string(res1.Body) {
		t.Fatalf("replay mismatch: %+v vs %+v", res2, res1)
	}
	if atomic.LoadInt32(&runs) != 1 {
		t.Fatalf("want single fn run, got %d", runs)
	}

	_, err = store.Execute(ctx, ik, h2, func() (*idempotency.HTTPResult, error) {
		t.Fatal("conflict must not invoke fn")
		return nil, nil
	})
	if !errors.Is(err, idempotency.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func deletePrefix(t *testing.T, cli *clientv3.Client, prefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := cli.Delete(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		t.Logf("cleanup delete prefix %q: %v", prefix, err)
	}
}
