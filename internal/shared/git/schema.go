package git

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

func isXApiGatewayEmpty(x XApiGatewaySchema) bool {
	return x.Prefix == "" && x.Contract.Name == "" && x.Service.Name == ""
}

func parseContractSchema(filename string, content []byte) (*ContractSchema, error) {
	ext := filepath.Ext(filename)

	switch ext {
	case ".json":
		slog.Debug("parsing contract schema file", "path", filename, "format", "json")
		return parseJSON(content)
	case ".yaml", ".yml":
		slog.Debug("parsing contract schema file", "path", filename, "format", "yaml")
		return parseYAML(content)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

func parseJSON(content []byte) (*ContractSchema, error) {
	var doc ContractSchema
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func parseYAML(content []byte) (*ContractSchema, error) {
	var contract ContractSchema
	if err := yaml.Unmarshal(content, &contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

func isSchemaFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".yml")
}
