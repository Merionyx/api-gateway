package kvvalue

import (
	"encoding/json"
	"fmt"
)

type schemaPeek struct {
	SchemaVersion *int `json:"schema_version"`
}

func peekPositiveSchemaVersion(data []byte) (int, error) {
	var p schemaPeek
	if err := json.Unmarshal(data, &p); err != nil {
		return 0, fmt.Errorf("kvvalue: json: %w", err)
	}
	if p.SchemaVersion == nil || *p.SchemaVersion < 1 {
		return 0, ErrMissingSchemaVersion
	}
	return *p.SchemaVersion, nil
}
