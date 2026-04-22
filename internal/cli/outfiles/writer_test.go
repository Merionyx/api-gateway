package outfiles

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	apiclient "github.com/merionyx/api-gateway/internal/cli/apiserver"
)

func TestSanitizeBase(t *testing.T) {
	t.Parallel()
	if g := sanitizeBase("a/b\\c"); g != "a_b_c" {
		t.Fatalf("%q", g)
	}
}

func TestWriteExported_noFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	raw := []byte("hello")
	files := []apiclient.ExportFile{{
		ContractName:  "c/name",
		SourcePath:    "x.yaml",
		ContentBase64: base64.StdEncoding.EncodeToString(raw),
	}}
	if err := WriteExported(files, dir, ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "c_name.yaml"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("%s %v", b, err)
	}
}

func TestWriteExported_yamlFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	doc := `x-api-gateway:
  version: v1
  prefix: /p/
  allowUndefinedMethods: false
  contract:
    name: c1
  service:
    name: s1
  access:
    secure: false
`
	files := []apiclient.ExportFile{{
		ContractName:  "export-me",
		SourcePath:    "b.yaml",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(doc)),
	}}
	if err := WriteExported(files, dir, "yaml"); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "export-me.*"))
	if len(matches) != 1 {
		t.Fatalf("got %v", matches)
	}
}
