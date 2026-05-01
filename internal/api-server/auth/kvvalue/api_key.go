package kvvalue

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	APIKeySchemaV1 = 1
	APIKeySchemaV2 = 2
)

// APIKeyValue is the canonical api-key record at api-keys/sha256/{digest_hex}.
type APIKeyValue struct {
	SchemaVersion int `json:"schema_version"`

	// Algorithm documents how digest_hex in the key path was produced (e.g. "sha256").
	Algorithm string `json:"algorithm"`

	Roles    []string        `json:"roles"`
	Scopes   []string        `json:"scopes"`
	Metadata json.RawMessage `json:"metadata,omitempty"`

	// RecordFormat is v2+ (e.g. "digest_v1"); migrated v1 defaults to DefaultAPIKeyRecordFormat.
	RecordFormat string `json:"record_format"`
}

const DefaultAPIKeyRecordFormat = "digest_v1"

type apiKeyValueV1Wire struct {
	SchemaVersion int             `json:"schema_version"`
	Algorithm     string          `json:"algorithm"`
	Roles         []string        `json:"roles"`
	Scopes        []string        `json:"scopes"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

func migrateAPIKeyV1(v1 apiKeyValueV1Wire) APIKeyValue {
	return APIKeyValue{
		SchemaVersion: APIKeySchemaV2,
		Algorithm:     v1.Algorithm,
		Roles:         append([]string(nil), v1.Roles...),
		Scopes:        append([]string(nil), v1.Scopes...),
		Metadata:      cloneRaw(v1.Metadata),
		RecordFormat:  DefaultAPIKeyRecordFormat,
	}
}

// ParseAPIKeyValueJSON parses JSON and supports known api-key schema versions.
func ParseAPIKeyValueJSON(data []byte) (APIKeyValue, error) {
	ver, err := peekPositiveSchemaVersion(data)
	if err != nil {
		return APIKeyValue{}, err
	}
	switch ver {
	case APIKeySchemaV1:
		var v1 apiKeyValueV1Wire
		if err := json.Unmarshal(data, &v1); err != nil {
			return APIKeyValue{}, fmt.Errorf("kvvalue: api-key v1: %w", err)
		}
		if v1.SchemaVersion != APIKeySchemaV1 {
			return APIKeyValue{}, ErrMissingSchemaVersion
		}
		return migrateAPIKeyV1(v1), nil
	case APIKeySchemaV2:
		var v2 APIKeyValue
		if err := json.Unmarshal(data, &v2); err != nil {
			return APIKeyValue{}, fmt.Errorf("kvvalue: api-key v2: %w", err)
		}
		if v2.SchemaVersion != APIKeySchemaV2 {
			return APIKeyValue{}, ErrMissingSchemaVersion
		}
		if v2.RecordFormat == "" {
			v2.RecordFormat = DefaultAPIKeyRecordFormat
		}
		return v2, nil
	default:
		return APIKeyValue{}, fmt.Errorf("%w: %d", ErrUnsupportedAPIKeySchema, ver)
	}
}

// MarshalAPIKeyValueJSON serializes for etcd Put with schema_version=2.
func MarshalAPIKeyValueJSON(v APIKeyValue) ([]byte, error) {
	if v.Algorithm == "" {
		return nil, errors.New("kvvalue: api-key algorithm required")
	}
	v.SchemaVersion = APIKeySchemaV2
	if v.RecordFormat == "" {
		v.RecordFormat = DefaultAPIKeyRecordFormat
	}
	return json.Marshal(v)
}
