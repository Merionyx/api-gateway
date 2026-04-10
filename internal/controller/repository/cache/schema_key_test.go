package cache

import (
	"fmt"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/repository/etcd"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

func TestParseSchemaContractEtcdKey(t *testing.T) {
	// Repository is a single path segment in etcd keys (slashes live in escaped ref/path).
	repo, ref, path := "schemas", "feature%2Ffoo", "pkg%2Fapi"
	key := fmt.Sprintf("%s%s/%s/%s/contracts/my-contract/snapshot",
		etcd.SchemaPrefix, repo, ref, path)

	gotRepo, gotRef, gotPath, ok := ParseSchemaContractEtcdKey(key)
	if !ok {
		t.Fatal("expected ok")
	}
	if gotRepo != "schemas" || gotRef != "feature/foo" || gotPath != "pkg/api" {
		t.Fatalf("got (%q,%q,%q)", gotRepo, gotRef, gotPath)
	}

	bk, ok := BundleKeyFromSchemaEtcdKey(key)
	if !ok {
		t.Fatal("BundleKeyFromSchemaEtcdKey")
	}
	wantBK := bundlekey.Build("schemas", "feature/foo", "pkg/api")
	if bk != wantBK {
		t.Errorf("bundle key %q want %q", bk, wantBK)
	}
}

func TestParseSchemaContractEtcdKey_rootPathDot(t *testing.T) {
	key := etcd.SchemaPrefix + "org/main/./contracts/c/snapshot"
	gotRepo, gotRef, gotPath, ok := ParseSchemaContractEtcdKey(key)
	if !ok {
		t.Fatal("expected ok")
	}
	if gotRepo != "org" || gotRef != "main" || gotPath != "" {
		t.Fatalf("got (%q,%q,%q)", gotRepo, gotRef, gotPath)
	}
}

func TestParseSchemaContractEtcdKey_rejections(t *testing.T) {
	bad := []string{
		"",
		"/other/prefix/schemas/x/y/z/contracts/c/snapshot",
		etcd.SchemaPrefix + "only/three/parts",
		etcd.SchemaPrefix + "a/b/c/d/contracts/c/wrong",
	}
	for _, k := range bad {
		if _, _, _, ok := ParseSchemaContractEtcdKey(k); ok {
			t.Errorf("expected false for %q", k)
		}
		if _, ok := BundleKeyFromSchemaEtcdKey(k); ok {
			t.Errorf("BundleKey expected false for %q", k)
		}
	}
}
