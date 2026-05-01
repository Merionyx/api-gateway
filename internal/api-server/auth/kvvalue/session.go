package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Session schema versions for etcd value at sessions/{session_id}.
const (
	SessionSchemaV1 = 1
)

// SessionValue is the canonical session value for schema v1.
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

	// ProviderID is the configured OIDC provider used for this session (IdP token refresh).
	ProviderID string `json:"provider_id"`

	// OurRefreshVerifier is an opaque verifier for the current our-refresh chain (hash/HMAC handle);
	// plaintext our refresh must not appear in etcd.
	OurRefreshVerifier string `json:"our_refresh_verifier,omitempty"`

	// RefreshExpiresAt is the absolute deadline for our refresh chain.
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
}

// ParseSessionValueJSON parses JSON from etcd and accepts only schema v1.
func ParseSessionValueJSON(data []byte) (SessionValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return SessionValue{}, err
	}
	switch ver {
	case SessionSchemaV1:
		var v1 SessionValue
		if err := json.Unmarshal(data, &v1); err != nil {
			return SessionValue{}, fmt.Errorf("kvvalue: session v1: %w", err)
		}
		if v1.SchemaVersion != SessionSchemaV1 {
			return SessionValue{}, ErrMissingSchemaVersion
		}
		if len(v1.EncryptedIDPRefresh) == 0 {
			return SessionValue{}, errors.New("kvvalue: session v1 encrypted_idp_refresh required")
		}
		if v1.ProviderID == "" {
			return SessionValue{}, errors.New("kvvalue: session v1 provider_id required")
		}
		return v1, nil
	default:
		return SessionValue{}, fmt.Errorf("%w: %d", ErrUnsupportedSessionSchema, ver)
	}
}

// MarshalSessionValueJSON serializes for etcd Put with schema_version=1.
func MarshalSessionValueJSON(s SessionValue) ([]byte, error) {
	if len(s.EncryptedIDPRefresh) == 0 {
		return nil, errors.New("kvvalue: session encrypted_idp_refresh required")
	}
	if s.ProviderID == "" {
		return nil, errors.New("kvvalue: session provider_id required")
	}
	s.SchemaVersion = SessionSchemaV1
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
