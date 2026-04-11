package bundle

import (
	"context"
	"errors"
	"testing"

	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type stubExportRemote struct {
	out []sharedgit.ExportedContractFile
	err error
}

func (s *stubExportRemote) ExportContractFiles(context.Context, string, string, string, string) ([]sharedgit.ExportedContractFile, error) {
	return s.out, s.err
}

func TestContractExportUseCase_Export(t *testing.T) {
	t.Parallel()
	u := NewContractExportUseCase(&stubExportRemote{out: []sharedgit.ExportedContractFile{{ContractName: "x"}}})
	out, err := u.Export(context.Background(), "r", "ref", "p", "c")
	if err != nil || len(out) != 1 || out[0].ContractName != "x" {
		t.Fatalf("got %v %#v", err, out)
	}

	u2 := NewContractExportUseCase(&stubExportRemote{err: errors.New("e")})
	_, err = u2.Export(context.Background(), "r", "ref", "p", "c")
	if err == nil {
		t.Fatal("expected error")
	}
}
