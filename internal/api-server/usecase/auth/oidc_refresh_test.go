package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/idpcache"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/sessioncrypto"
	"github.com/merionyx/api-gateway/internal/api-server/config"
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
	st, code, detail := MapRefreshError(fmt.Errorf("%w: provider detail", oidc.ErrMissingRefreshTokenInTokenResponse))
	if st != http.StatusUnauthorized || code != "OIDC_REFRESH_TOKEN_UNAVAILABLE" || detail != "provider detail" {
		t.Fatalf("got %d %s %q", st, code, detail)
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

type memRefreshSessionStore struct {
	mu        sync.Mutex
	sessionID string
	sess      kvvalue.SessionValue
	modRev    int64
	verifier  string
}

func (m *memRefreshSessionStore) Get(_ context.Context, sessionID string) (kvvalue.SessionValue, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sessionID != m.sessionID {
		return kvvalue.SessionValue{}, 0, apierrors.ErrNotFound
	}
	return m.sess, m.modRev, nil
}

func (m *memRefreshSessionStore) GetSessionIDByRefreshVerifier(_ context.Context, v string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v != m.verifier {
		return "", apierrors.ErrNotFound
	}
	return m.sessionID, nil
}

func (m *memRefreshSessionStore) ReplaceCASWithRefreshIndex(_ context.Context, sessionID, oldVerifier, newVerifier string, v kvvalue.SessionValue, expectedModRevision int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sessionID != m.sessionID || oldVerifier != m.verifier || expectedModRevision != m.modRev {
		return fmt.Errorf("cas mismatch")
	}
	m.sess = v
	m.modRev++
	m.verifier = newVerifier
	return nil
}

func TestOIDCRefresh_degraded_discovery503(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	kr, err := sessioncrypto.NewKeyring(sessioncrypto.KEK{ID: "k", Key: k})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	sealed, err := kr.Seal([]byte("idp-rt"))
	if err != nil {
		t.Fatal(err)
	}
	envBytes, err := sessioncrypto.MarshalEnvelope(sealed)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{
		"email":   "user@example.com",
		"roles":   []any{"admin"},
		"idp_iss": "http://issuer",
		"idp_sub": "idp-sub-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	ourBytes := make([]byte, 32)
	if _, err := rand.Read(ourBytes); err != nil {
		t.Fatal(err)
	}
	ourHex := hex.EncodeToString(ourBytes)
	verifier := ourOpaqueRefreshVerifier(ourHex)

	sess := kvvalue.SessionValue{
		SchemaVersion:       kvvalue.SessionSchemaV2,
		EncryptedIDPRefresh: json.RawMessage(envBytes),
		ClaimsSnapshot:      claims,
		RotationGeneration:  2,
		LoginIntentID:       "intent-1",
		ProviderID:          "p1",
		OurRefreshVerifier:  verifier,
	}
	st := &memRefreshSessionStore{
		sessionID: "sid-1",
		sess:      sess,
		modRev:    7,
		verifier:  verifier,
	}

	cache := idpcache.New(nil)
	cache.Put("sid-1", "stale-idp-access", time.Hour)

	uc := NewOIDCRefreshUseCase([]config.OIDCProviderConfig{{
		ID:           "p1",
		Name:         "Test Provider",
		Issuer:       srv.URL,
		ClientID:     "cid",
		ClientSecret: "sec",
	}}, st, kr, jwtUC, srv.Client(), 5*time.Minute, false, cache, 0)

	out, err := uc.Refresh(context.Background(), ourHex)
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatalf("tokens %+v", out)
	}
	if out.RefreshToken == ourHex {
		t.Fatal("our refresh should rotate")
	}
	st.mu.Lock()
	got := st.sess
	st.mu.Unlock()
	if got.RotationGeneration != 3 {
		t.Fatalf("rotation gen %d", got.RotationGeneration)
	}
	if string(got.EncryptedIDPRefresh) != string(envBytes) {
		t.Fatal("encrypted idp refresh should be unchanged on degraded path")
	}
	if string(got.ClaimsSnapshot) != string(claims) {
		t.Fatal("claims snapshot unchanged")
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(out.AccessToken, jwt.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	mc, _ := parsed.Claims.(jwt.MapClaims)
	if mc["sub"] != "user@example.com" {
		t.Fatalf("sub %v", mc["sub"])
	}
	roles, _ := mc["roles"].([]any)
	if len(roles) != 1 || roles[0] != "admin" {
		t.Fatalf("roles %+v", roles)
	}
	if mc["idp_iss"] != "http://issuer" || mc["idp_sub"] != "idp-sub-1" {
		t.Fatalf("idp claims %+v", mc)
	}
	if _, ok := cache.Get("sid-1"); ok {
		t.Fatal("expected idp access cache cleared after refresh CAS (degraded does not repopulate)")
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

func TestOIDCRefresh_missingStoredIDPRefreshToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	kr, err := sessioncrypto.NewKeyring(sessioncrypto.KEK{ID: "k", Key: k})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	sealed, err := kr.Seal([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	envBytes, err := sessioncrypto.MarshalEnvelope(sealed)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{
		"email": "user@gmail.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	ourBytes := make([]byte, 32)
	if _, err := rand.Read(ourBytes); err != nil {
		t.Fatal(err)
	}
	ourHex := hex.EncodeToString(ourBytes)
	verifier := ourOpaqueRefreshVerifier(ourHex)

	st := &memRefreshSessionStore{
		sessionID: "sid-1",
		sess: kvvalue.SessionValue{
			SchemaVersion:       kvvalue.SessionSchemaV2,
			EncryptedIDPRefresh: json.RawMessage(envBytes),
			ClaimsSnapshot:      claims,
			ProviderID:          "google",
			OurRefreshVerifier:  verifier,
		},
		modRev:   7,
		verifier: verifier,
	}

	uc := NewOIDCRefreshUseCase([]config.OIDCProviderConfig{{
		ID:       "google",
		Name:     "Google",
		Kind:     "google",
		Issuer:   "https://accounts.google.com",
		ClientID: "cid",
	}}, st, kr, jwtUC, http.DefaultClient, 5*time.Minute, false, nil, 0)

	_, err = uc.Refresh(context.Background(), ourHex)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, oidc.ErrMissingRefreshTokenInTokenResponse) {
		t.Fatalf("got %v", err)
	}
	stCode, code, detail := MapRefreshError(err)
	if stCode != http.StatusUnauthorized || code != "OIDC_REFRESH_TOKEN_UNAVAILABLE" {
		t.Fatalf("got %d %s %q", stCode, code, detail)
	}
	if !strings.Contains(detail, "Google did not issue a refresh token") {
		t.Fatalf("detail %q", detail)
	}
}
