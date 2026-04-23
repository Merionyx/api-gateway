package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Session schema versions for etcd value at sessions/{session_id}.
const (
	SessionSchemaV1     = 1
	SessionSchemaV2     = 2
	SessionSchemaLatest = SessionSchemaV2
)

// SessionValue is the canonical session value (latest schema) after ParseSessionValueJSON.
// Field names are snake_case for JSON (roadmap).
type SessionValue struct {
	SchemaVersion int `json:"schema_version"`

	// EncryptedIDPRefresh is the opaque ciphertext blob (e.g. sessioncrypto.Envelope JSON) — not plaintext IdP refresh.
	EncryptedIDPRefresh json.RawMessage `json:"encrypted_idp_refresh"`

	// ClaimsSnapshot is the last successfully reconciled claims/roles JSON (IdP up); may be empty object.
	ClaimsSnapshot json.RawMessage `json:"claims_snapshot,omitempty"`

	// RotationGeneration is introduced in v2; v1 migrations set it to 0.
	RotationGeneration int64 `json:"rotation_generation"`

	// LoginIntentID links to login-intents/{intent_id} established this session (roadmap ш. 13–14).
	LoginIntentID string `json:"login_intent_id,omitempty"`

	// OurRefreshVerifier is an opaque verifier for the current our-refresh chain (hash/HMAC handle);
	// plaintext our refresh must not appear in etcd (roadmap п. 13).
	OurRefreshVerifier string `json:"our_refresh_verifier,omitempty"`
}

type sessionValueV1Wire struct {
	SchemaVersion       int             `json:"schema_version"`
	EncryptedIDPRefresh json.RawMessage `json:"encrypted_idp_refresh"`
	ClaimsSnapshot      json.RawMessage `json:"claims_snapshot,omitempty"`
}

func migrateSessionV1(v1 sessionValueV1Wire) SessionValue {
	return SessionValue{
		SchemaVersion:       SessionSchemaLatest,
		EncryptedIDPRefresh: cloneRaw(v1.EncryptedIDPRefresh),
		ClaimsSnapshot:      cloneRaw(v1.ClaimsSnapshot),
		RotationGeneration:  0,
	}
}

// ParseSessionValueJSON parses JSON from etcd and migrates v1 → latest on read.
func ParseSessionValueJSON(data []byte) (SessionValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return SessionValue{}, err
	}
	switch ver {
	case SessionSchemaV1:
		var v1 sessionValueV1Wire
		if err := json.Unmarshal(data, &v1); err != nil {
			return SessionValue{}, fmt.Errorf("kvvalue: session v1: %w", err)
		}
		if v1.SchemaVersion != SessionSchemaV1 {
			return SessionValue{}, ErrMissingSchemaVersion
		}
		if len(v1.EncryptedIDPRefresh) == 0 {
			return SessionValue{}, errors.New("kvvalue: session v1 encrypted_idp_refresh required")
		}
		out := migrateSessionV1(v1)
		return out, nil
	case SessionSchemaV2:
		var v2 SessionValue
		if err := json.Unmarshal(data, &v2); err != nil {
			return SessionValue{}, fmt.Errorf("kvvalue: session v2: %w", err)
		}
		if v2.SchemaVersion != SessionSchemaV2 {
			return SessionValue{}, ErrMissingSchemaVersion
		}
		if len(v2.EncryptedIDPRefresh) == 0 {
			return SessionValue{}, errors.New("kvvalue: session v2 encrypted_idp_refresh required")
		}
		return v2, nil
	default:
		return SessionValue{}, fmt.Errorf("%w: %d", ErrUnsupportedSessionSchema, ver)
	}
}

// MarshalSessionValueJSON serializes for etcd Put (always latest schema_version).
func MarshalSessionValueJSON(s SessionValue) ([]byte, error) {
	if len(s.EncryptedIDPRefresh) == 0 {
		return nil, errors.New("kvvalue: session encrypted_idp_refresh required")
	}
	s.SchemaVersion = SessionSchemaLatest
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
