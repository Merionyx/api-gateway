package contractfmt

import "gopkg.in/yaml.v3"

// x-api-gateway v1: canonical key order (orderedKinMap + slices below).
// Keep in sync with validate/xagw.go. XAGDoc encoding preserves all keys at every nesting level.

var (
	xagV1Keys = []string{
		"version",
		"prefix",
		"allowUndefinedMethods",
		"contract",
		"service",
		"access",
	}
	xagContractKeys = []string{"name", "description"}
	xagServiceKeys  = []string{"name"}
	xagAccessKeys   = []string{"secure", "apps"}
	xagAppKeys      = []string{"app_id", "environments"}
)

// Typed wrappers so encodeValueOrdered applies XAG key order instead of generic map/JSON-schema heuristics.
type (
	xagContractMap map[string]any
	xagServiceMap  map[string]any
	xagAccessMap   map[string]any
	xagAppMap      map[string]any
)

// encodeXAGDoc emits the full x-api-gateway tree with canonical key order at each known level; unknown keys remain (sorted last per level).
func encodeXAGDoc(d XAGDoc) (*yaml.Node, error) {
	if d == nil {
		return encodeNullNode(), nil
	}
	m := map[string]any(d)
	prepared := make(map[string]any, len(m))
	for k, v := range m {
		prepared[k] = wrapXAGChild(k, v)
	}
	return orderedKinMap(prepared, xagV1Keys)
}

func wrapXAGChild(k string, v any) any {
	switch k {
	case "contract":
		if mm, ok := v.(map[string]any); ok {
			return xagContractMap(mm)
		}
	case "service":
		if mm, ok := v.(map[string]any); ok {
			return xagServiceMap(mm)
		}
	case "access":
		if mm, ok := v.(map[string]any); ok {
			return wrapXAGAccessMap(mm)
		}
	}
	return v
}

func wrapXAGAccessMap(m map[string]any) xagAccessMap {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k != "apps" {
			out[k] = v
			continue
		}
		arr, ok := v.([]any)
		if !ok {
			out[k] = v
			continue
		}
		apps := make([]any, len(arr))
		for i, el := range arr {
			if mm, ok := el.(map[string]any); ok {
				apps[i] = xagAppMap(mm)
			} else {
				apps[i] = el
			}
		}
		out[k] = apps
	}
	return xagAccessMap(out)
}

func encodeXAGContractMap(m xagContractMap) (*yaml.Node, error) {
	return orderedKinMap(map[string]any(m), xagContractKeys)
}

func encodeXAGServiceMap(m xagServiceMap) (*yaml.Node, error) {
	return orderedKinMap(map[string]any(m), xagServiceKeys)
}

func encodeXAGAccessMap(m xagAccessMap) (*yaml.Node, error) {
	return orderedKinMap(map[string]any(m), xagAccessKeys)
}

func encodeXAGAppMap(m xagAppMap) (*yaml.Node, error) {
	return orderedKinMap(map[string]any(m), xagAppKeys)
}

func encodeXAGV1(x *XAGV1) (*yaml.Node, error) {
	if x == nil {
		return encodeNullNode(), nil
	}
	m := map[string]any{
		"version":               x.Version,
		"prefix":                x.Prefix,
		"allowUndefinedMethods": x.AllowUndefinedMethods,
		"contract":              x.Contract,
		"service":               x.Service,
		"access":                x.Access,
	}
	return orderedKinMap(m, xagV1Keys)
}

func encodeXAGContract(c XAGContract) (*yaml.Node, error) {
	m := map[string]any{
		"name": c.Name,
	}
	if c.Description != "" {
		m["description"] = c.Description
	}
	return orderedKinMap(m, xagContractKeys)
}

func encodeXAGService(s XAGService) (*yaml.Node, error) {
	m := map[string]any{
		"name": s.Name,
	}
	return orderedKinMap(m, xagServiceKeys)
}

func encodeXAGAccess(a XAGAccess) (*yaml.Node, error) {
	m := map[string]any{
		"secure": a.Secure,
	}
	if len(a.Apps) > 0 {
		m["apps"] = a.Apps
	}
	return orderedKinMap(m, xagAccessKeys)
}

func encodeXAGApp(a XAGApp) (*yaml.Node, error) {
	m := map[string]any{
		"app_id": a.AppID,
	}
	if len(a.Environments) > 0 {
		m["environments"] = a.Environments
	}
	return orderedKinMap(m, xagAppKeys)
}

func encodeXAGAppSlice(apps []XAGApp) (*yaml.Node, error) {
	var seq yaml.Node
	seq.Kind = yaml.SequenceNode
	for i := range apps {
		n, err := encodeXAGApp(apps[i])
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, n)
	}
	return &seq, nil
}
