//go:build integration

package integration

import (
	"context"
	"testing"

	apiserveretcd "merionyx/api-gateway/internal/api-server/repository/etcd"
	sharedgit "merionyx/api-gateway/internal/shared/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestSaveSnapshots_roundTripAndPrune(t *testing.T) {
	cli := NewEtcdClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	const bundleKey = "integration_snapshot_bundle"
	prefix := "/api-gateway/api-server/snapshots/" + bundleKey + "/"
	ctx := context.Background()
	_, err := cli.Delete(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = cli.Delete(context.Background(), prefix, clientv3.WithPrefix())
	})

	repo := apiserveretcd.NewSnapshotRepository(cli)
	a := sharedgit.ContractSnapshot{Name: "a", Prefix: "/a"}
	b := sharedgit.ContractSnapshot{Name: "b", Prefix: "/b"}

	written, err := repo.SaveSnapshots(ctx, bundleKey, []sharedgit.ContractSnapshot{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected first save to write")
	}

	written, err = repo.SaveSnapshots(ctx, bundleKey, []sharedgit.ContractSnapshot{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Fatal("identical payload should not write")
	}

	written, err = repo.SaveSnapshots(ctx, bundleKey, []sharedgit.ContractSnapshot{a})
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("prune should delete b and report written")
	}

	snaps, err := repo.GetSnapshots(ctx, bundleKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Name != "a" {
		t.Fatalf("got %+v", snaps)
	}
}
