package validate

import (
	"fmt"
	"strings"
)

const extKey = "x-api-gateway"

// ValidateXApiGateway checks presence and shape of x-api-gateway for the declared schema version (currently v1 only).
func ValidateXApiGateway(root map[string]any) []string {
	raw, ok := root[extKey]
	if !ok {
		return []string{fmt.Sprintf("[%s] missing top-level %q extension", extKey, extKey)}
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("[%s] must be a mapping, got %T", extKey, raw)}
	}
	ver, err := stringField(block, "version")
	if err != nil {
		return []string{fmt.Sprintf("[%s] %v", extKey, err)}
	}
	switch strings.TrimSpace(ver) {
	case "v1":
		return validateXAGv1(block)
	default:
		return []string{fmt.Sprintf("[%s] unsupported schema version %q (supported: v1)", extKey, ver)}
	}
}

func validateXAGv1(x map[string]any) []string {
	var out []string
	add := func(msg string) { out = append(out, fmt.Sprintf("[x-api-gateway] %s", msg)) }

	if p, err := stringField(x, "prefix"); err != nil {
		add(err.Error())
	} else if strings.TrimSpace(p) == "" {
		add(`field "prefix" must be non-empty`)
	}
	if err := requireBool(x, "allowUndefinedMethods"); err != nil {
		add(err.Error())
	}
	if err := validateContract(x); err != nil {
		add(err.Error())
	}
	if err := validateService(x); err != nil {
		add(err.Error())
	}
	if err := validateAccess(x); err != nil {
		add(err.Error())
	}
	return out
}

func validateContract(x map[string]any) error {
	raw, ok := x["contract"]
	if !ok {
		return fmt.Errorf(`field "contract" is required`)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf(`field "contract" must be a mapping, got %T`, raw)
	}
	name, err := stringField(m, "name")
	if err != nil {
		return fmt.Errorf(`contract: %w`, err)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf(`contract.name must be non-empty`)
	}
	if d, ok := m["description"]; ok && d != nil {
		if _, ok := d.(string); !ok {
			return fmt.Errorf(`contract.description must be a string, got %T`, d)
		}
	}
	return nil
}

func validateService(x map[string]any) error {
	raw, ok := x["service"]
	if !ok {
		return fmt.Errorf(`field "service" is required`)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf(`field "service" must be a mapping, got %T`, raw)
	}
	name, err := stringField(m, "name")
	if err != nil {
		return fmt.Errorf(`service: %w`, err)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf(`service.name must be non-empty`)
	}
	return nil
}

func validateAccess(x map[string]any) error {
	raw, ok := x["access"]
	if !ok {
		return fmt.Errorf(`field "access" is required`)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf(`field "access" must be a mapping, got %T`, raw)
	}
	if _, err := requireBoolMap(m, "secure"); err != nil {
		return fmt.Errorf(`access: %w`, err)
	}
	apps, ok := m["apps"]
	if !ok || apps == nil {
		return nil
	}
	arr, ok := apps.([]any)
	if !ok {
		return fmt.Errorf(`access.apps must be a sequence, got %T`, apps)
	}
	for i, it := range arr {
		am, ok := it.(map[string]any)
		if !ok {
			return fmt.Errorf(`access.apps[%d] must be a mapping, got %T`, i, it)
		}
		if _, err := stringField(am, "app_id"); err != nil {
			return fmt.Errorf(`access.apps[%d]: %w`, i, err)
		}
		if envs, ok := am["environments"]; ok && envs != nil {
			el, ok := envs.([]any)
			if !ok {
				return fmt.Errorf(`access.apps[%d].environments must be a sequence, got %T`, i, envs)
			}
			for j, e := range el {
				if _, ok := e.(string); !ok {
					return fmt.Errorf(`access.apps[%d].environments[%d] must be a string, got %T`, i, j, e)
				}
			}
		}
	}
	return nil
}

func stringField(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("field %q is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q must be a string, got %T", key, v)
	}
	return s, nil
}

func requireBool(m map[string]any, key string) error {
	_, err := requireBoolMap(m, key)
	return err
}

func requireBoolMap(m map[string]any, key string) (bool, error) {
	v, ok := m[key]
	if !ok {
		return false, fmt.Errorf("field %q is required", key)
	}
	b, ok := asBool(v)
	if !ok {
		return false, fmt.Errorf("field %q must be a boolean, got %T", key, v)
	}
	return b, nil
}

func asBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	default:
		return false, false
	}
}
