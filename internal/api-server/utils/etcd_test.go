package utils

import "testing"

func TestExtractRepoRefContractFromKey(t *testing.T) {
	t.Parallel()
	// Layout: /api-gateway/api-server/{repo}/{ref}/contracts/{contract} — see adapter/etcd snapshot keys.
	key := []byte("/api-gateway/api-server/acme%2Fsvc/feature%2Fbranch/contracts/openapi.yaml")
	repo, ref, contract := ExtractRepoRefContractFromKey(key)
	if repo != "acme%2Fsvc" || ref != "feature/branch" || contract != "openapi.yaml" {
		t.Fatalf("got repo=%q ref=%q contract=%q", repo, ref, contract)
	}
}
