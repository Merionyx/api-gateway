package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsContractFile(t *testing.T) {
	t.Parallel()
	if !isContractFile("x.YAML") || !isContractFile("a.json") {
		t.Fatal("expected yaml/json")
	}
	if isContractFile("readme.md") {
		t.Fatal("md should not match")
	}
}

func TestCollectFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(f, []byte("k: v"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := CollectFiles(f)
	if err != nil || len(paths) != 1 || paths[0] != f {
		t.Fatalf("file target: %v %#v", err, paths)
	}
	paths, err = CollectFiles(dir)
	if err != nil || len(paths) != 1 {
		t.Fatalf("dir walk: %v %#v", err, paths)
	}
	if _, err := CollectFiles(filepath.Join(dir, "nope.yaml")); err == nil {
		t.Fatal("missing file")
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err = CollectFiles(dir)
	if err != nil || len(paths) != 2 {
		t.Fatalf("nested: %v %#v", err, paths)
	}
}
