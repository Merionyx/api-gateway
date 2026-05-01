package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Session schema versions for etcd value at sessions/{session_id}.
const (
	SessionSchemaV3 = 3
)

// SessionValue is the canonical session value for schema v3.
// Field names are snake_case for JSON.
type SessionValue struct {
	SchemaVersion int `json:"schema_version"`

	// EncryptedIDPRefresh is the opaque ciphertext blob (e.g. sessioncrypto.Envelope JSON) — not plaintext IdP refresh.
	EncryptedIDPRefresh json.RawMessage `json:"encrypted_idp_refresh"`

	// ClaimsSnapshot is the last successfully reconciled claims/roles JSON (IdP up); may be empty object.
	ClaimsSnapshot json.RawMessage `json:"claims_snapshot,omitempty"`

	// RotationGeneration increments on each refresh rotation.
	RotationGeneration int64 `json:"rotation_generation"`

	// LoginIntentID links to login-intents/{intent_id} established this session.
	LoginIntentID string `json:"login_intent_id,omitempty"`

	// ProviderID is the configured OIDC provider used for this session (IdP token refresh;).
	ProviderID string `json:"provider_id,omitempty"`

	// OurRefreshVerifier is an opaque verifier for the current our-refresh chain (hash/HMAC handle);
	// plaintext our refresh must not appear in etcd.
	OurRefreshVerifier string `json:"our_refresh_verifier,omitempty"`

	// RefreshExpiresAt is the absolute deadline for our refresh chain.
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
}

// ParseSessionValueJSON parses JSON from etcd and accepts only schema v3.
func ParseSessionValueJSON(data []byte) (SessionValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return SessionValue{}, err
	}
	switch ver {
	case SessionSchemaV3:
		var v3 SessionValue
		if err := json.Unmarshal(data, &v3); err != nil {
			return SessionValue{}, fmt.Errorf("kvvalue: session v3: %w", err)
		}
		if v3.SchemaVersion != SessionSchemaV3 {
			return SessionValue{}, ErrMissingSchemaVersion
		}
		if len(v3.EncryptedIDPRefresh) == 0 {
			return SessionValue{}, errors.New("kvvalue: session v3 encrypted_idp_refresh required")
		}
		return v3, nil
	default:
		return SessionValue{}, fmt.Errorf("%w: %d", ErrUnsupportedSessionSchema, ver)
	}
}

// MarshalSessionValueJSON serializes for etcd Put with schema_version=3.
func MarshalSessionValueJSON(s SessionValue) ([]byte, error) {
	if len(s.EncryptedIDPRefresh) == 0 {
		return nil, errors.New("kvvalue: session encrypted_idp_refresh required")
	}
	s.SchemaVersion = SessionSchemaV3
	return json.Marshal(s)
}

func cloneRaw(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return nil
	}
	out := make([]byte, len(r))
	copy(out, r)
	return out
}
