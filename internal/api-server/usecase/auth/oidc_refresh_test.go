package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func TestOurOpaqueRefreshVerifier_matchesCallbackPattern(t *testing.T) {
	t.Parallel()
	ourHex := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	sum := sha256.Sum256([]byte(ourHex))
	want := hex.EncodeToString(sum[:])
	if g := ourOpaqueRefreshVerifier(ourHex); g != want {
		t.Fatalf("got %s want %s", g, want)
	}
}

func TestSubjectFromClaimsSnapshot(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(map[string]any{"email": "a@b.c", "sub": "sub1"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := subjectFromClaimsSnapshot(raw)
	if err != nil || s != "a@b.c" {
		t.Fatalf("got %q err %v", s, err)
	}
}

func TestMapRefreshError(t *testing.T) {
	t.Parallel()
	st, code, _ := MapRefreshError(apierrors.ErrSessionRefreshConflict)
	if st != http.StatusConflict || code != "REFRESH_STATE_CONFLICT" {
		t.Fatalf("got %d %s", st, code)
	}
	st, _, _ = MapRefreshError(errors.New("opaque"))
	if st != http.StatusInternalServerError {
		t.Fatalf("got %d", st)
	}
}

func TestSubjectFromClaimsSnapshot_empty(t *testing.T) {
	t.Parallel()
	_, err := subjectFromClaimsSnapshot(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionValue_providerID_roundTrip(t *testing.T) {
	t.Parallel()
	s := kvvalue.SessionValue{
		SchemaVersion:       kvvalue.SessionSchemaV2,
		EncryptedIDPRefresh: json.RawMessage(`{"v":1}`),
		ProviderID:          "p1",
		OurRefreshVerifier:  strings.Repeat("a", 64),
	}
	b, err := kvvalue.MarshalSessionValueJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kvvalue.ParseSessionValueJSON(b)
	if err != nil || got.ProviderID != "p1" {
		t.Fatalf("%+v err %v", got, err)
	}
}
