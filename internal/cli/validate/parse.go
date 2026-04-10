package validate

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseRoot parses YAML or JSON (YAML is a superset for our use) into a map.
func ParseRoot(data []byte) (map[string]any, error) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("empty document")
	}
	return root, nil
}
