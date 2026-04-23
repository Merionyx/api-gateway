package kvvalue

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSessionMigrateV1ToV2OnRead(t *testing.T) {
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

func TestSessionV2RoundTrip(t *testing.T) {
	t.Parallel()
	s := SessionValue{
		SchemaVersion:       SessionSchemaV2,
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

func TestLoginIntentMigrateV1(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version": 1,
		"provider_id": "gitlab",
		"redirect_uri": "https://cb",
		"oauth_state": "st",
		"pkce_verifier": "ver"
	}`)
	got, err := ParseLoginIntentValueJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != LoginIntentSchemaLatest {
		t.Fatal("schema")
	}
	if got.IntentProtocol != DefaultIntentProtocol {
		t.Fatalf("intent_protocol: %q", got.IntentProtocol)
	}
	b, err := MarshalLoginIntentValueJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	got2, err := ParseLoginIntentValueJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if got2.IntentProtocol != DefaultIntentProtocol {
		t.Fatal(got2.IntentProtocol)
	}
}

func TestLoginIntentMarshalRequiresFields(t *testing.T) {
	t.Parallel()
	_, err := MarshalLoginIntentValueJSON(LoginIntentValue{
		SchemaVersion: 2,
		ProviderID:    "x",
	})
	if err == nil {
		t.Fatal("expected error")
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
