package storage

import (
	"testing"

	authv1 "github.com/merionyx/api-gateway/pkg/grpc/auth/v1"
)

func TestAccessStorage_FindContractByPath_longestPrefixWins(t *testing.T) {
	s := NewAccessStorage()
	s.SetAccessConfig(&authv1.AccessConfig{
		Version: 1,
		Contracts: []*authv1.ContractAccess{
			{ContractName: "short", Prefix: "/api"},
			{ContractName: "long", Prefix: "/api/v1"},
		},
	})

	got := s.FindContractByPath("/api/v1/users")
	if got == nil || got.ContractName != "long" {
		t.Fatalf("want long contract, got %v", got)
	}

	got2 := s.FindContractByPath("/api/other")
	if got2 == nil || got2.ContractName != "short" {
		t.Fatalf("want short contract, got %v", got2)
	}
}

func TestAccessStorage_ApplyUpdate_rebuildsPrefixOrder(t *testing.T) {
	s := NewAccessStorage()
	s.SetAccessConfig(&authv1.AccessConfig{
		Version: 1,
		Contracts: []*authv1.ContractAccess{
			{ContractName: "a", Prefix: "/x"},
		},
	})
	s.ApplyUpdate(&authv1.AccessUpdate{
		Version: 2,
		AddedContracts: []*authv1.ContractAccess{
			{ContractName: "b", Prefix: "/x/y"},
		},
	})

	got := s.FindContractByPath("/x/y/z")
	if got == nil || got.ContractName != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestAccessStorage_ApplyUpdate_removeClearsPrefix(t *testing.T) {
	s := NewAccessStorage()
	s.SetAccessConfig(&authv1.AccessConfig{
		Version: 1,
		Contracts: []*authv1.ContractAccess{
			{ContractName: "rm", Prefix: "/p"},
		},
	})
	s.ApplyUpdate(&authv1.AccessUpdate{
		Version:          2,
		RemovedContracts: []string{"rm"},
	})
	if s.FindContractByPath("/p/a") != nil {
		t.Fatal("expected no match after remove")
	}
}
