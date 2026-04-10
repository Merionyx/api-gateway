package contractfmt

import (
	"fmt"
)

// XAGV1 is the canonical x-api-gateway v1 block (field order matches YAML emission order).
type XAGV1 struct {
	Version               string      `yaml:"version"`
	Prefix                string      `yaml:"prefix"`
	AllowUndefinedMethods bool        `yaml:"allowUndefinedMethods"`
	Contract              XAGContract `yaml:"contract"`
	Service               XAGService  `yaml:"service"`
	Access                XAGAccess   `yaml:"access"`
}

// XAGContract is contract metadata under x-api-gateway.
type XAGContract struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// XAGService names the backing service.
type XAGService struct {
	Name string `yaml:"name"`
}

// XAGAccess describes gateway access rules.
type XAGAccess struct {
	Secure bool     `yaml:"secure"`
	Apps   []XAGApp `yaml:"apps,omitempty"`
}

// XAGApp binds an app to environments.
type XAGApp struct {
	AppID        string   `yaml:"app_id"`
	Environments []string `yaml:"environments,omitempty"`
}

// XAGDoc is the raw x-api-gateway mapping (full tree). The formatter preserves every key at every level.
type XAGDoc map[string]any

// ParseXAGV1 builds a typed view from a generic mapping (unknown keys are ignored).
func ParseXAGV1(m map[string]any) (*XAGV1, error) {
	if m == nil {
		return nil, fmt.Errorf("x-api-gateway: nil mapping")
	}
	var x XAGV1
	x.Version = stringFromAny(m["version"])
	x.Prefix = stringFromAny(m["prefix"])
	x.AllowUndefinedMethods = boolFromAny(m["allowUndefinedMethods"])
	if cm, ok := m["contract"].(map[string]any); ok {
		x.Contract = parseXAGContract(cm)
	}
	if sm, ok := m["service"].(map[string]any); ok {
		x.Service = parseXAGService(sm)
	}
	if am, ok := m["access"].(map[string]any); ok {
		a, err := parseXAGAccess(am)
		if err != nil {
			return nil, err
		}
		x.Access = a
	}
	return &x, nil
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func boolFromAny(v any) bool {
	b, ok := v.(bool)
	if ok {
		return b
	}
	return false
}

func parseXAGContract(m map[string]any) XAGContract {
	return XAGContract{
		Name:        stringFromAny(m["name"]),
		Description: stringFromAny(m["description"]),
	}
}

func parseXAGService(m map[string]any) XAGService {
	return XAGService{Name: stringFromAny(m["name"])}
}

func parseXAGAccess(m map[string]any) (XAGAccess, error) {
	var a XAGAccess
	a.Secure = boolFromAny(m["secure"])
	raw, ok := m["apps"]
	if !ok || raw == nil {
		return a, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return a, fmt.Errorf("x-api-gateway: access.apps must be a sequence")
	}
	for i, el := range arr {
		appm, ok := el.(map[string]any)
		if !ok {
			return a, fmt.Errorf("x-api-gateway: access.apps[%d] must be a mapping", i)
		}
		app, err := parseXAGApp(appm)
		if err != nil {
			return a, err
		}
		a.Apps = append(a.Apps, app)
	}
	return a, nil
}

func parseXAGApp(m map[string]any) (XAGApp, error) {
	var a XAGApp
	a.AppID = stringFromAny(m["app_id"])
	if raw, ok := m["environments"]; ok && raw != nil {
		arr, ok := raw.([]any)
		if !ok {
			return a, fmt.Errorf("x-api-gateway: app.environments must be a sequence")
		}
		for _, el := range arr {
			s, ok := el.(string)
			if !ok {
				return a, fmt.Errorf("x-api-gateway: app.environments entries must be strings")
			}
			a.Environments = append(a.Environments, s)
		}
	}
	return a, nil
}
