package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	LoginIntentSchemaV1 = 1
)

// LoginIntentValue is the canonical login-intent value at login-intents/{intent_id}.
type LoginIntentValue struct {
	SchemaVersion int `json:"schema_version"`

	ProviderID   string `json:"provider_id"`
	RedirectURI  string `json:"redirect_uri"`
	OAuthState   string `json:"oauth_state"`
	PKCEVerifier string `json:"pkce_verifier"`

	// IntentProtocol is the login intent protocol marker (e.g. "oidc_v1").
	IntentProtocol string `json:"intent_protocol"`

	// Nonce is optional OIDC nonce for id_token validation at callback.
	Nonce string `json:"nonce,omitempty"`

	// OAuthClientID is the downstream OAuth client_id from GET /v1/auth/authorize.
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

// ParseLoginIntentValueJSON parses JSON and accepts only schema v1.
func ParseLoginIntentValueJSON(data []byte) (LoginIntentValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return LoginIntentValue{}, err
	}
	switch ver {
	case LoginIntentSchemaV1:
		var v1 LoginIntentValue
		if err := json.Unmarshal(data, &v1); err != nil {
			return LoginIntentValue{}, fmt.Errorf("kvvalue: login-intent v1: %w", err)
		}
		if v1.SchemaVersion != LoginIntentSchemaV1 {
			return LoginIntentValue{}, ErrMissingSchemaVersion
		}
		if v1.ProviderID == "" || v1.RedirectURI == "" || v1.OAuthState == "" || v1.PKCEVerifier == "" {
			return LoginIntentValue{}, errors.New("kvvalue: login-intent required string fields missing")
		}
		if v1.IntentProtocol == "" {
			return LoginIntentValue{}, errors.New("kvvalue: login-intent intent_protocol required")
		}
		return v1, nil
	default:
		return LoginIntentValue{}, fmt.Errorf("%w: %d", ErrUnsupportedLoginIntentSchema, ver)
	}
}

// MarshalLoginIntentValueJSON serializes for etcd Put with schema_version=1.
func MarshalLoginIntentValueJSON(v LoginIntentValue) ([]byte, error) {
	if v.ProviderID == "" || v.RedirectURI == "" || v.OAuthState == "" || v.PKCEVerifier == "" {
		return nil, errors.New("kvvalue: login-intent required string fields missing")
	}
	if v.IntentProtocol == "" {
		return nil, errors.New("kvvalue: login-intent intent_protocol required")
	}
	v.SchemaVersion = LoginIntentSchemaV1
	return json.Marshal(v)
}
