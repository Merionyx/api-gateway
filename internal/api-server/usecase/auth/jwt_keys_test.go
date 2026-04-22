package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJWTUseCase_loadKeys_skipsInvalidThenGenerates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "garbage.key"), []byte("not valid pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	uc, err := NewJWTUseCase(dir, "iss")
	if err != nil {
		t.Fatal(err)
	}
	if len(uc.GetSigningKeys(context.Background())) == 0 {
		t.Fatal("expected generated default key")
	}
}
