package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestSessionMigrateV1ToLatestOnRead(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version": 1,
		"encrypted_idp_refresh": {"v":1,"alg":"AES-256-GCM-envelope-v1"},
		"claims_snapshot": {"roles":["x"]}
	}`)
	got, err := ParseSessionValueJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SessionSchemaLatest {
		t.Fatalf("schema: want %d got %d", SessionSchemaLatest, got.SchemaVersion)
	}
	if got.RotationGeneration != 0 {
		t.Fatalf("rotation_generation: want 0 after migrate, got %d", got.RotationGeneration)
	}
	var env map[string]any
	if err := json.Unmarshal(got.EncryptedIDPRefresh, &env); err != nil {
		t.Fatal(err)
	}
	if env["v"].(float64) != 1 {
		t.Fatalf("encrypted blob: %v", env)
	}
}

func TestSessionMigrateV2ToLatestOnRead(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version": 2,
		"encrypted_idp_refresh": {"k":1},
		"claims_snapshot": {"roles":["x"]},
		"rotation_generation": 5,
		"login_intent_id": "6ba7b810-9dad-41d4-a716-446655440001",
		"provider_id": "p1",
		"our_refresh_verifier": "opaque-verifier-handle"
	}`)
	got, err := ParseSessionValueJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SessionSchemaLatest {
		t.Fatalf("schema: want %d got %d", SessionSchemaLatest, got.SchemaVersion)
	}
	if got.RotationGeneration != 5 || got.ProviderID != "p1" || got.OurRefreshVerifier != "opaque-verifier-handle" {
		t.Fatalf("migrated session: %+v", got)
	}
	if !got.RefreshExpiresAt.IsZero() {
		t.Fatalf("legacy v2 must not invent refresh expiry: %s", got.RefreshExpiresAt)
	}
}

func TestSessionLatestOptionalFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	s := SessionValue{
		SchemaVersion:       SessionSchemaLatest,
		EncryptedIDPRefresh: json.RawMessage(`{"k":1}`),
		RotationGeneration:  1,
		LoginIntentID:       "6ba7b810-9dad-41d4-a716-446655440001",
		OurRefreshVerifier:  "opaque-verifier-handle",
	}
	b, err := MarshalSessionValueJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSessionValueJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.LoginIntentID != s.LoginIntentID || got.OurRefreshVerifier != s.OurRefreshVerifier {
		t.Fatalf("optional fields lost: %+v", got)
	}
}

func TestSessionLatestRoundTrip(t *testing.T) {
	t.Parallel()
	s := SessionValue{
		SchemaVersion:       SessionSchemaLatest,
		EncryptedIDPRefresh: json.RawMessage(`{}`),
		ClaimsSnapshot:      json.RawMessage(`[]`),
		RotationGeneration:  3,
	}
	b, err := MarshalSessionValueJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSessionValueJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.RotationGeneration != 3 {
		t.Fatalf("rotation: %d", got.RotationGeneration)
	}
	if got.SchemaVersion != SessionSchemaLatest {
		t.Fatal("schema")
	}
}

func TestSessionMarshalRequiresEncryptedBlob(t *testing.T) {
	t.Parallel()
	_, err := MarshalSessionValueJSON(SessionValue{SchemaVersion: 2})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionUnsupportedVersion(t *testing.T) {
	t.Parallel()
	_, err := ParseSessionValueJSON([]byte(`{"schema_version":99,"encrypted_idp_refresh":{}}`))
	if !errors.Is(err, ErrUnsupportedSessionSchema) {
		t.Fatalf("got %v", err)
	}
}

func TestSessionMissingSchema(t *testing.T) {
	t.Parallel()
	_, err := ParseSessionValueJSON([]byte(`{}`))
	if !errors.Is(err, ErrMissingSchemaVersion) {
		t.Fatalf("got %v", err)
	}
}

func TestLoginIntentLatestRoundTrip(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version": 4,
		"provider_id": "gitlab",
		"redirect_uri": "https://cb",
		"oauth_state": "st",
		"pkce_verifier": "ver",
		"intent_protocol": "oidc_v1",
		"nonce": "n1",
		"oauth_client_id": "postman",
		"oauth_client_redirect_uri": "https://oauth.pstmn.io/v1/callback",
		"oauth_client_state": "st2",
		"oauth_client_code_challenge": "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~",
		"oauth_client_code_challenge_method": "S256"
	}`)
	got, err := ParseLoginIntentValueJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != LoginIntentSchemaLatest {
		t.Fatal("schema")
	}
	if got.Nonce != "n1" {
		t.Fatalf("nonce %q", got.Nonce)
	}
	if got.OAuthClientID != "postman" {
		t.Fatalf("oauth_client_id %q", got.OAuthClientID)
	}
}

func TestLoginIntentRejectsLegacySchemas(t *testing.T) {
	t.Parallel()
	for _, ver := range []int{1, 2, 3} {
		raw := []byte(fmt.Sprintf(`{"schema_version":%d}`, ver))
		_, err := ParseLoginIntentValueJSON(raw)
		if !errors.Is(err, ErrUnsupportedLoginIntentSchema) {
			t.Fatalf("schema_version=%d: got %v", ver, err)
		}
	}
}

func TestLoginIntentMarshalRequiresFields(t *testing.T) {
	t.Parallel()
	_, err := MarshalLoginIntentValueJSON(LoginIntentValue{
		SchemaVersion: LoginIntentSchemaLatest,
		ProviderID:    "x",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoginIntentNonceRoundtrip(t *testing.T) {
	t.Parallel()
	v := LoginIntentValue{
		SchemaVersion:                  LoginIntentSchemaLatest,
		ProviderID:                     "p",
		RedirectURI:                    "https://a/cb",
		OAuthState:                     "st",
		PKCEVerifier:                   "pv",
		IntentProtocol:                 DefaultIntentProtocol,
		Nonce:                          "n-1",
		OAuthClientID:                  "postman",
		OAuthClientRedirectURI:         "https://oauth.pstmn.io/v1/callback",
		OAuthClientState:               "client-state-1",
		OAuthClientCodeChallenge:       "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~",
		OAuthClientCodeChallengeMethod: "S256",
	}
	raw, err := MarshalLoginIntentValueJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseLoginIntentValueJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Nonce != "n-1" {
		t.Fatalf("nonce %q", got.Nonce)
	}
	if got.OAuthClientID != "postman" || got.OAuthClientRedirectURI != "https://oauth.pstmn.io/v1/callback" {
		t.Fatalf("oauth client fields %+v", got)
	}
	if got.OAuthClientCodeChallengeMethod != "S256" || got.OAuthClientCodeChallenge == "" {
		t.Fatalf("oauth pkce fields %+v", got)
	}
}

func TestAPIKeyMigrateV1(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version": 1,
		"algorithm": "sha256",
		"roles": ["a"],
		"scopes": ["b"],
		"metadata": {"k":"v"}
	}`)
	got, err := ParseAPIKeyValueJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != APIKeySchemaLatest {
		t.Fatal("schema")
	}
	if got.RecordFormat != DefaultAPIKeyRecordFormat {
		t.Fatal(got.RecordFormat)
	}
	b, err := MarshalAPIKeyValueJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	got2, err := ParseAPIKeyValueJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if got2.RecordFormat != DefaultAPIKeyRecordFormat {
		t.Fatal(got2.RecordFormat)
	}
}

func TestAPIKeyUnsupportedVersion(t *testing.T) {
	t.Parallel()
	_, err := ParseAPIKeyValueJSON([]byte(`{"schema_version":5,"algorithm":"sha256"}`))
	if !errors.Is(err, ErrUnsupportedAPIKeySchema) {
		t.Fatalf("got %v", err)
	}
}
