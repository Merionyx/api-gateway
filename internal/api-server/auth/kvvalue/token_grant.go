package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	TokenGrantSchemaV1     = 1
	TokenGrantSchemaLatest = TokenGrantSchemaV1
)

// TokenGrantValue stores server-side delegated permissions for one API access token (jti).
type TokenGrantValue struct {
	SchemaVersion int       `json:"schema_version"`
	Permissions   []string  `json:"permissions"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// ParseTokenGrantValueJSON parses JSON from etcd.
func ParseTokenGrantValueJSON(data []byte) (TokenGrantValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return TokenGrantValue{}, err
	}
	switch ver {
	case TokenGrantSchemaV1:
		var v TokenGrantValue
		if err := json.Unmarshal(data, &v); err != nil {
			return TokenGrantValue{}, fmt.Errorf("kvvalue: token-grant v1: %w", err)
		}
		if v.SchemaVersion != TokenGrantSchemaV1 {
			return TokenGrantValue{}, ErrMissingSchemaVersion
		}
		if len(v.Permissions) == 0 {
			return TokenGrantValue{}, errors.New("kvvalue: token-grant permissions required")
		}
		if v.ExpiresAt.IsZero() {
			return TokenGrantValue{}, errors.New("kvvalue: token-grant expires_at required")
		}
		return v, nil
	default:
		return TokenGrantValue{}, fmt.Errorf("%w: %d", ErrUnsupportedTokenGrantSchema, ver)
	}
}

// MarshalTokenGrantValueJSON serializes for etcd Put (always latest schema_version).
func MarshalTokenGrantValueJSON(v TokenGrantValue) ([]byte, error) {
	if len(v.Permissions) == 0 {
		return nil, errors.New("kvvalue: token-grant permissions required")
	}
	if v.ExpiresAt.IsZero() {
		return nil, errors.New("kvvalue: token-grant expires_at required")
	}
	v.SchemaVersion = TokenGrantSchemaLatest
	return json.Marshal(v)
}
