package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	APIKeySchemaV1 = 1
)

// APIKeyValue is the canonical api-key record at api-keys/sha256/{digest_hex}.
type APIKeyValue struct {
	SchemaVersion int `json:"schema_version"`

	// Algorithm documents how digest_hex in the key path was produced (e.g. "sha256").
	Algorithm string `json:"algorithm"`

	Roles    []string        `json:"roles"`
	Scopes   []string        `json:"scopes"`
	Metadata json.RawMessage `json:"metadata,omitempty"`

	// RecordFormat marks how digest_hex in the key path was produced (e.g. "digest_v1").
	RecordFormat string `json:"record_format"`
}

const DefaultAPIKeyRecordFormat = "digest_v1"

// ParseAPIKeyValueJSON parses JSON and accepts only schema v1.
func ParseAPIKeyValueJSON(data []byte) (APIKeyValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return APIKeyValue{}, err
	}
	switch ver {
	case APIKeySchemaV1:
		var v1 APIKeyValue
		if err := json.Unmarshal(data, &v1); err != nil {
			return APIKeyValue{}, fmt.Errorf("kvvalue: api-key v1: %w", err)
		}
		if v1.SchemaVersion != APIKeySchemaV1 {
			return APIKeyValue{}, ErrMissingSchemaVersion
		}
		if v1.RecordFormat == "" {
			v1.RecordFormat = DefaultAPIKeyRecordFormat
		}
		return v1, nil
	default:
		return APIKeyValue{}, fmt.Errorf("%w: %d", ErrUnsupportedAPIKeySchema, ver)
	}
}

// MarshalAPIKeyValueJSON serializes for etcd Put with schema_version=1.
func MarshalAPIKeyValueJSON(v APIKeyValue) ([]byte, error) {
	if v.Algorithm == "" {
		return nil, errors.New("kvvalue: api-key algorithm required")
	}
	v.SchemaVersion = APIKeySchemaV1
	if v.RecordFormat == "" {
		v.RecordFormat = DefaultAPIKeyRecordFormat
	}
	return json.Marshal(v)
}
