//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdPutGetRoundTrip(t *testing.T) {
	cli := NewEtcdClient(t)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "/api-gateway/integration-test/roundtrip-" + t.Name()
	val := "v1"
	_, err := cli.Put(ctx, key, val)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := cli.Get(ctx, key, clientv3.WithLimit(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Kvs) != 1 || string(resp.Kvs[0].Value) != val {
		t.Fatalf("got %+v", resp)
	}
	_, _ = cli.Delete(ctx, key)
}
