//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/repository/cache"
	ctrlrepoetcd "github.com/merionyx/api-gateway/internal/controller/repository/etcd"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func schemaBundlePrefix(repo, ref, bundlePath string) string {
	return fmt.Sprintf("%s%s/%s/%s/", ctrlrepoetcd.SchemaPrefix, repo, bundlekey.EscapeRef(ref), bundlekey.EscapePath(bundlePath))
}

func schemaContractKey(repo, ref, bundlePath, contract string) string {
	return fmt.Sprintf("%s%s/%s/%s/contracts/%s/snapshot",
		ctrlrepoetcd.SchemaPrefix,
		repo,
		bundlekey.EscapeRef(ref),
		bundlekey.EscapePath(bundlePath),
		contract,
	)
}

func TestSchemaRepository_listAndCache(t *testing.T) {
	cli := NewEtcdClient(t)
	t.Cleanup(func() { _ = cli.Close() })

	repo := "itestschema"
	ref := "main"
	bundlePath := "pkg"
	prefix := schemaBundlePrefix(repo, ref, bundlePath)
	ctx := context.Background()
	_, err := cli.Delete(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = cli.Delete(context.Background(), prefix, clientv3.WithPrefix())
	})

	schema := ctrlrepoetcd.NewSchemaRepository(cli)
	snap := &models.ContractSnapshot{Name: "c1", Prefix: "/api"}

	if err := schema.SaveContractSnapshot(ctx, repo, ref, bundlePath, "c1", snap); err != nil {
		t.Fatal(err)
	}

	list1, err := schema.ListContractSnapshots(ctx, repo, ref, bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(list1) != 1 || list1[0].Name != "c1" {
		t.Fatalf("list1 %+v", list1)
	}

	wrapped := cache.NewSchemaCache(schema, false)
	if _, err := wrapped.ListContractSnapshots(ctx, repo, ref, bundlePath); err != nil {
		t.Fatal(err)
	}
	if _, err := wrapped.ListContractSnapshots(ctx, repo, ref, bundlePath); err != nil {
		t.Fatal(err)
	}

	key := schemaContractKey(repo, ref, bundlePath, "c1")
	eff := cache.ClassifyControllerEtcdWatchKey(key)
	if eff.SchemaBundleKey == "" {
		t.Fatalf("expected schema bundle effect for %q", key)
	}

	wrapped.InvalidateBundleKey(eff.SchemaBundleKey)
	list4, err := wrapped.ListContractSnapshots(ctx, repo, ref, bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(list4) != 1 {
		t.Fatal("after invalidate")
	}
}
