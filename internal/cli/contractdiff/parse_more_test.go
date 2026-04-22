package contractdiff

import "testing"

func TestParseDocument_yamlJsonYmlAndErrors(t *testing.T) {
	t.Parallel()
	if _, err := ParseDocument([]byte("not: valid: :"), ".yaml"); err == nil {
		t.Fatal("want yaml error")
	}
	if _, err := ParseDocument([]byte("not json"), ".json"); err == nil {
		t.Fatal("want json error")
	}
	root, err := ParseDocument([]byte(`{"openapi":"3"}`), ".json")
	if err != nil {
		t.Fatal(err)
	}
	if root == nil {
		t.Fatal("nil")
	}
	y, err := ParseDocument([]byte("a: 1\n"), ".YML")
	if err != nil || y["a"] == nil {
		t.Fatalf("yml normalize: %v", err)
	}
	if _, err := ParseDocument([]byte("x"), ".md"); err == nil {
		t.Fatal("bad ext")
	}
}

func TestExtFromPath(t *testing.T) {
	t.Parallel()
	if ExtFromPath("/a/B.C") != ".c" {
		t.Fatal()
	}
}

func TestContractNameFromDoc_errors(t *testing.T) {
	t.Parallel()
	if _, err := ContractNameFromDoc(map[string]any{}); err == nil {
		t.Fatal()
	}
	if _, err := ContractNameFromDoc(map[string]any{"x-api-gateway": 1}); err == nil {
		t.Fatal()
	}
	if _, err := ContractNameFromDoc(map[string]any{
		"x-api-gateway": map[string]any{},
	}); err == nil {
		t.Fatal("missing contract")
	}
	if _, err := ContractNameFromDoc(map[string]any{
		"x-api-gateway": map[string]any{"contract": 1},
	}); err == nil {
		t.Fatal("contract not map")
	}
	if _, err := ContractNameFromDoc(map[string]any{
		"x-api-gateway": map[string]any{"contract": map[string]any{"name": ""}},
	}); err == nil {
		t.Fatal("empty name")
	}
}
