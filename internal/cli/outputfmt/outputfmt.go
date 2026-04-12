// Package outputfmt encodes CLI output for structured API responses.
package outputfmt

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format selects how to print a value.
type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
)

// Parse normalizes -o / --output values (json, yaml; table is default).
func Parse(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "table", "wide":
		return Table, nil
	case "json":
		return JSON, nil
	case "yaml", "yml":
		return YAML, nil
	default:
		return "", fmt.Errorf("unknown output format %q (supported: table, json, yaml)", s)
	}
}

// Write marshals v to w according to format (not used for Table — callers print tables themselves).
func Write(w io.Writer, f Format, v any) error {
	switch f {
	case Table:
		return fmt.Errorf("outputfmt.Write: use table printers for format table")
	case JSON:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = w.Write(append(b, '\n'))
		return err
	case YAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		defer func() { _ = enc.Close() }()
		return enc.Encode(v)
	default:
		return fmt.Errorf("unsupported format %q", f)
	}
}

// String returns kubectl-style flag value for help text.
func (f Format) String() string {
	switch f {
	case Table:
		return "table"
	case JSON:
		return "json"
	case YAML:
		return "yaml"
	default:
		return string(f)
	}
}
