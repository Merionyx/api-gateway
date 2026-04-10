package contractdiff

import "testing"

func TestContractNameFromDoc(t *testing.T) {
	root := map[string]any{
		"x-api-gateway": map[string]any{
			"contract": map[string]any{"name": "my-contract"},
		},
	}
	n, err := ContractNameFromDoc(root)
	if err != nil {
		t.Fatal(err)
	}
	if n != "my-contract" {
		t.Fatalf("got %q", n)
	}
}

func TestCanonYAMLStringRoundTrip(t *testing.T) {
	doc := map[string]any{
		"openapi": "3.0.0",
		"x-api-gateway": map[string]any{
			"version": "v1",
			"contract": map[string]any{
				"name": "c",
			},
		},
	}
	s, err := canonYAMLString(doc)
	if err != nil {
		t.Fatal(err)
	}
	if s == "" {
		t.Fatal("empty yaml")
	}
}
