package bundle

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type stubSnapRepo struct {
	keys  []string
	snaps []sharedgit.ContractSnapshot
	err   error
}

func (s *stubSnapRepo) ListBundleKeys(context.Context) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.keys, nil
}

func (s *stubSnapRepo) GetSnapshots(context.Context, string) ([]sharedgit.ContractSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.snaps, nil
}

func (s *stubSnapRepo) SaveSnapshots(context.Context, string, []sharedgit.ContractSnapshot) (bool, error) {
	panic("unexpected")
}

func TestBundleReadUseCase_ListBundleKeys(t *testing.T) {
	t.Parallel()
	u := NewBundleReadUseCase(&stubSnapRepo{keys: []string{"b", "a"}})
	out, _, _, err := u.ListBundleKeys(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %#v", out)
	}

	u2 := NewBundleReadUseCase(&stubSnapRepo{err: errors.New("e")})
	_, _, _, err = u2.ListBundleKeys(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBundleReadUseCase_ListContractNames(t *testing.T) {
	t.Parallel()
	u := NewBundleReadUseCase(&stubSnapRepo{
		snaps: []sharedgit.ContractSnapshot{
			{Name: "z"},
			{Name: "a"},
			{Name: "a"},
		},
	})
	names, _, _, err := u.ListContractNames(context.Background(), "bk", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "z" {
		t.Fatalf("got %#v", names)
	}
}

func TestBundleReadUseCase_GetContractDocument(t *testing.T) {
	t.Parallel()
	u := NewBundleReadUseCase(&stubSnapRepo{
		snaps: []sharedgit.ContractSnapshot{{Name: "api", Prefix: "p"}},
	})
	doc, err := u.GetContractDocument(context.Background(), "bk", "api")
	if err != nil {
		t.Fatal(err)
	}
	if doc["prefix"] != "p" {
		t.Fatalf("got %#v", doc)
	}

	_, err = u.GetContractDocument(context.Background(), "bk", "missing")
	if !errors.Is(err, apierrors.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
