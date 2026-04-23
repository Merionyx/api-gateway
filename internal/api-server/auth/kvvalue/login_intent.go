package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	LoginIntentSchemaV1     = 1
	LoginIntentSchemaV2     = 2
	LoginIntentSchemaLatest = LoginIntentSchemaV2
)

// LoginIntentValue is the canonical login-intent value at login-intents/{intent_id}.
type LoginIntentValue struct {
	SchemaVersion int `json:"schema_version"`

	ProviderID   string `json:"provider_id"`
	RedirectURI  string `json:"redirect_uri"`
	OAuthState   string `json:"oauth_state"`
	PKCEVerifier string `json:"pkce_verifier"`

	// IntentProtocol is v2+ (e.g. "oidc_v1"); migrated v1 defaults to DefaultIntentProtocol.
	IntentProtocol string `json:"intent_protocol"`

	// Nonce is optional OIDC nonce for id_token validation at callback (roadmap ш. 13–14).
	Nonce string `json:"nonce,omitempty"`
}

const DefaultIntentProtocol = "oidc_v1"

type loginIntentV1Wire struct {
	SchemaVersion int    `json:"schema_version"`
	ProviderID    string `json:"provider_id"`
	RedirectURI   string `json:"redirect_uri"`
	OAuthState    string `json:"oauth_state"`
	PKCEVerifier  string `json:"pkce_verifier"`
}

func migrateLoginIntentV1(v1 loginIntentV1Wire) LoginIntentValue {
	return LoginIntentValue{
		SchemaVersion:   LoginIntentSchemaLatest,
		ProviderID:      v1.ProviderID,
		RedirectURI:     v1.RedirectURI,
		OAuthState:      v1.OAuthState,
		PKCEVerifier:    v1.PKCEVerifier,
		IntentProtocol:  DefaultIntentProtocol,
	}
}

// ParseLoginIntentValueJSON parses JSON and migrates v1 → latest on read.
func ParseLoginIntentValueJSON(data []byte) (LoginIntentValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return LoginIntentValue{}, err
	}
	switch ver {
	case LoginIntentSchemaV1:
		var v1 loginIntentV1Wire
		if err := json.Unmarshal(data, &v1); err != nil {
			return LoginIntentValue{}, fmt.Errorf("kvvalue: login-intent v1: %w", err)
		}
		if v1.SchemaVersion != LoginIntentSchemaV1 {
			return LoginIntentValue{}, ErrMissingSchemaVersion
		}
		return migrateLoginIntentV1(v1), nil
	case LoginIntentSchemaV2:
		var v2 LoginIntentValue
		if err := json.Unmarshal(data, &v2); err != nil {
			return LoginIntentValue{}, fmt.Errorf("kvvalue: login-intent v2: %w", err)
		}
		if v2.SchemaVersion != LoginIntentSchemaV2 {
			return LoginIntentValue{}, ErrMissingSchemaVersion
		}
		if v2.IntentProtocol == "" {
			v2.IntentProtocol = DefaultIntentProtocol
		}
		return v2, nil
	default:
		return LoginIntentValue{}, fmt.Errorf("%w: %d", ErrUnsupportedLoginIntentSchema, ver)
	}
}

// MarshalLoginIntentValueJSON serializes for etcd Put (always latest schema_version).
func MarshalLoginIntentValueJSON(v LoginIntentValue) ([]byte, error) {
	if v.ProviderID == "" || v.RedirectURI == "" || v.OAuthState == "" || v.PKCEVerifier == "" {
		return nil, errors.New("kvvalue: login-intent required string fields missing")
	}
	v.SchemaVersion = LoginIntentSchemaLatest
	if v.IntentProtocol == "" {
		v.IntentProtocol = DefaultIntentProtocol
	}
	return json.Marshal(v)
}
