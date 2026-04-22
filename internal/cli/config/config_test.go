package config

import (
	"path/filepath"
	"testing"
)

func TestValidateContextName(t *testing.T) {
	t.Parallel()
	if err := ValidateContextName(""); err == nil {
		t.Fatal("empty")
	}
	if err := ValidateContextName("a/b"); err == nil {
		t.Fatal("slash")
	}
	if err := ValidateContextName("ok"); err != nil {
		t.Fatal(err)
	}
}

func TestFile_SetContext_UseContext_Delete(t *testing.T) {
	t.Parallel()
	f := &File{}
	if err := f.SetContext("c1", "https://a"); err != nil {
		t.Fatal(err)
	}
	if err := f.UseContext("c1"); err != nil {
		t.Fatal(err)
	}
	if f.CurrentContext != "c1" {
		t.Fatalf("%q", f.CurrentContext)
	}
	names := f.ContextNames()
	if len(names) != 1 || names[0] != "c1" {
		t.Fatalf("%#v", names)
	}
	ok, err := f.DeleteContext("c1")
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	_, _ = f.DeleteContext("c1")
}

func TestFile_ContextNames_nil(t *testing.T) {
	t.Parallel()
	if n := (*File)(nil).ContextNames(); n != nil {
		t.Fatalf("%#v", n)
	}
}

func TestLoadSave_roundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	t.Setenv("AGWCTL_CONFIG", p)
	f := &File{CurrentContext: "c", Contexts: map[string]ContextConfig{
		"c": {Server: "https://example"},
	}}
	if err := Save(f); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentContext != "c" || got.Contexts["c"].Server != "https://example" {
		t.Fatalf("%#v", got)
	}
}

func TestPath_env(t *testing.T) {
	t.Setenv("AGWCTL_CONFIG", "/tmp/cfg.yaml")
	p, err := Path()
	if err != nil || p != "/tmp/cfg.yaml" {
		t.Fatalf("%q %v", p, err)
	}
}

func TestResolveServerURL(t *testing.T) {
	t.Parallel()
	if u, err := ResolveServerURL("", "  https://x  "); err != nil || u != "https://x" {
		t.Fatalf("%q %v", u, err)
	}
}

func TestFile_UseContext_errors(t *testing.T) {
	t.Parallel()
	f := &File{Contexts: map[string]ContextConfig{"x": {Server: " "}}}
	if err := f.UseContext("x"); err == nil {
		t.Fatal("empty server")
	}
}
