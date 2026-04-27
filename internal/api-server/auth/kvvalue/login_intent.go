package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	LoginIntentSchemaV1     = 1
	LoginIntentSchemaV2     = 2
	LoginIntentSchemaV3     = 3
	LoginIntentSchemaV4     = 4
	LoginIntentSchemaLatest = LoginIntentSchemaV4
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

	// RequestedAccessTokenTTLSeconds is the client-requested access token lifetime for callback issuance.
	RequestedAccessTokenTTLSeconds int64 `json:"requested_access_token_ttl_seconds,omitempty"`
	// RequestedRefreshTokenTTLSeconds is the client-requested refresh-chain lifetime for callback issuance.
	RequestedRefreshTokenTTLSeconds int64 `json:"requested_refresh_token_ttl_seconds,omitempty"`

	// OAuthClientID is the downstream OAuth client_id from GET /api/v1/auth/authorize.
	OAuthClientID string `json:"oauth_client_id,omitempty"`
	// OAuthClientRedirectURI is the downstream OAuth redirect_uri used after IdP callback.
	OAuthClientRedirectURI string `json:"oauth_client_redirect_uri,omitempty"`
	// OAuthClientState is echoed to the downstream client redirect as state.
	OAuthClientState string `json:"oauth_client_state,omitempty"`
	// OAuthClientCodeChallenge is the downstream PKCE challenge to validate at token exchange.
	OAuthClientCodeChallenge string `json:"oauth_client_code_challenge,omitempty"`
	// OAuthClientCodeChallengeMethod stores downstream PKCE method; only S256 is accepted.
	OAuthClientCodeChallengeMethod string `json:"oauth_client_code_challenge_method,omitempty"`
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
		SchemaVersion:  LoginIntentSchemaLatest,
		ProviderID:     v1.ProviderID,
		RedirectURI:    v1.RedirectURI,
		OAuthState:     v1.OAuthState,
		PKCEVerifier:   v1.PKCEVerifier,
		IntentProtocol: DefaultIntentProtocol,
	}
}

type loginIntentV2Wire struct {
	SchemaVersion  int    `json:"schema_version"`
	ProviderID     string `json:"provider_id"`
	RedirectURI    string `json:"redirect_uri"`
	OAuthState     string `json:"oauth_state"`
	PKCEVerifier   string `json:"pkce_verifier"`
	IntentProtocol string `json:"intent_protocol"`
	Nonce          string `json:"nonce,omitempty"`
}

func migrateLoginIntentV2(v2 loginIntentV2Wire) LoginIntentValue {
	return LoginIntentValue{
		SchemaVersion:  LoginIntentSchemaLatest,
		ProviderID:     v2.ProviderID,
		RedirectURI:    v2.RedirectURI,
		OAuthState:     v2.OAuthState,
		PKCEVerifier:   v2.PKCEVerifier,
		IntentProtocol: v2.IntentProtocol,
		Nonce:          v2.Nonce,
	}
}

func migrateLoginIntentV3(v3 LoginIntentValue) LoginIntentValue {
	v3.SchemaVersion = LoginIntentSchemaLatest
	return v3
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
		var v2 loginIntentV2Wire
		if err := json.Unmarshal(data, &v2); err != nil {
			return LoginIntentValue{}, fmt.Errorf("kvvalue: login-intent v2: %w", err)
		}
		if v2.SchemaVersion != LoginIntentSchemaV2 {
			return LoginIntentValue{}, ErrMissingSchemaVersion
		}
		if v2.IntentProtocol == "" {
			v2.IntentProtocol = DefaultIntentProtocol
		}
		return migrateLoginIntentV2(v2), nil
	case LoginIntentSchemaV3:
		var v3 LoginIntentValue
		if err := json.Unmarshal(data, &v3); err != nil {
			return LoginIntentValue{}, fmt.Errorf("kvvalue: login-intent v3: %w", err)
		}
		if v3.SchemaVersion != LoginIntentSchemaV3 {
			return LoginIntentValue{}, ErrMissingSchemaVersion
		}
		if v3.IntentProtocol == "" {
			v3.IntentProtocol = DefaultIntentProtocol
		}
		return migrateLoginIntentV3(v3), nil
	case LoginIntentSchemaV4:
		var v4 LoginIntentValue
		if err := json.Unmarshal(data, &v4); err != nil {
			return LoginIntentValue{}, fmt.Errorf("kvvalue: login-intent v4: %w", err)
		}
		if v4.SchemaVersion != LoginIntentSchemaV4 {
			return LoginIntentValue{}, ErrMissingSchemaVersion
		}
		if v4.IntentProtocol == "" {
			v4.IntentProtocol = DefaultIntentProtocol
		}
		return v4, nil
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
