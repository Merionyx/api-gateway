package etcd

import (
	"encoding/json"
	"testing"

	sharedgit "merionyx/api-gateway/internal/shared/git"
)

func snap(name, prefix string) sharedgit.ContractSnapshot {
	return sharedgit.ContractSnapshot{
		Name:   name,
		Prefix: prefix,
	}
}

func TestBuildSnapshotSavePlan_noopWhenUnchanged(t *testing.T) {
	bundleKey := "org%2Frepo%2Fmain%2Epkg"
	s := snap("c1", "/p")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string][]byte{"c1": data}

	puts, dels, err := buildSnapshotSavePlan(bundleKey, existing, []sharedgit.ContractSnapshot{s})
	if err != nil {
		t.Fatal(err)
	}
	if len(puts) != 0 || len(dels) != 0 {
		t.Fatalf("expected noop, puts=%d dels=%d", len(puts), len(dels))
	}
}

func TestBuildSnapshotSavePlan_putNew(t *testing.T) {
	puts, dels, err := buildSnapshotSavePlan("bk", map[string][]byte{}, []sharedgit.ContractSnapshot{snap("n", "/a")})
	if err != nil {
		t.Fatal(err)
	}
	if len(puts) != 1 || len(dels) != 0 {
		t.Fatalf("puts=%d dels=%d", len(puts), len(dels))
	}
	wantKey := snapshotPrefix + "bk/contracts/n"
	if puts[0].key != wantKey {
		t.Errorf("key %q want %q", puts[0].key, wantKey)
	}
}

func TestBuildSnapshotSavePlan_deleteOrphan(t *testing.T) {
	puts, dels, err := buildSnapshotSavePlan("bk", map[string][]byte{"old": []byte(`{}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(puts) != 0 || len(dels) != 1 {
		t.Fatalf("puts=%d dels=%v", len(puts), dels)
	}
	want := snapshotPrefix + "bk/contracts/old"
	if dels[0] != want {
		t.Errorf("del key %q", dels[0])
	}
}

func TestBuildSnapshotSavePlan_putWhenContentChanges(t *testing.T) {
	old, err := json.Marshal(snap("c", "/x"))
	if err != nil {
		t.Fatal(err)
	}
	newS := snap("c", "/y")
	puts, dels, err := buildSnapshotSavePlan("bk", map[string][]byte{"c": old}, []sharedgit.ContractSnapshot{newS})
	if err != nil {
		t.Fatal(err)
	}
	if len(puts) != 1 || len(dels) != 0 {
		t.Fatalf("puts=%d dels=%d", len(puts), len(dels))
	}
	var decoded sharedgit.ContractSnapshot
	if err := json.Unmarshal([]byte(puts[0].val), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Prefix != "/y" {
		t.Fatalf("decoded %+v", decoded)
	}
}
