package git

import (
	"strings"
	"testing"
)

func TestParseContractSchema_UnsupportedExt(t *testing.T) {
	_, err := parseContractSchema("x.txt", []byte("{}"))
	if err == nil || !strings.Contains(err.Error(), "unsupported file format") {
		t.Fatalf("got %v", err)
	}
}

func TestParseContractSchema_InvalidJSON(t *testing.T) {
	_, err := parseContractSchema("a.json", []byte("not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseContractSchema_InvalidYAML(t *testing.T) {
	_, err := parseContractSchema("a.yaml", []byte(":\n  bad"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestContractSnapshotsFromRepoFiles_SkipsEmptyXApiGateway(t *testing.T) {
	yaml := `openapi: 3.0.0
info:
  title: t
  version: "1"
paths: {}
x-api-gateway: {}
`
	snaps, err := contractSnapshotsFromRepoFiles([]RepositoryFile{
		{Path: "empty.yaml", Content: []byte(yaml)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected no snapshots, got %d", len(snaps))
	}
}

func TestIsSchemaFile(t *testing.T) {
	if !isSchemaFile("a.yaml") || !isSchemaFile("b.yml") || !isSchemaFile("c.json") {
		t.Fatal("expected true")
	}
	if isSchemaFile("x.txt") {
		t.Fatal("expected false")
	}
}

func TestIsXApiGatewayEmpty(t *testing.T) {
	if !isXApiGatewayEmpty(XApiGatewaySchema{}) {
		t.Fatal("empty struct should be empty")
	}
	if isXApiGatewayEmpty(XApiGatewaySchema{Prefix: "/p"}) {
		t.Fatal("non-empty")
	}
}
