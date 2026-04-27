package handler

import (
	"encoding/json"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func subjectFromAPIJWTClaims(mc jwt.MapClaims) string {
	if e, _ := mc["email"].(string); strings.TrimSpace(e) != "" {
		return strings.TrimSpace(e)
	}
	if s, _ := mc["sub"].(string); strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return ""
}

func permissionsFromAPIJWTClaims(mc jwt.MapClaims) []any {
	return mergeAnyUnique(claimSliceToAny(mc, "permissions"), claimSliceToAny(mc, "scopes"))
}

func claimSliceToAny(mc jwt.MapClaims, key string) []any {
	v, ok := mc[key]
	if !ok || v == nil {
		return []any{}
	}
	switch x := v.(type) {
	case []any:
		return append([]any(nil), x...)
	case []string:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return []any{}
		}
		return []any{s}
	default:
		return []any{}
	}
}

func stringsToAny(in []string) []any {
	if len(in) == 0 {
		return []any{}
	}
	out := make([]any, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i := range in {
		s := strings.TrimSpace(in[i])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func mergeAnyUnique(base, add []any) []any {
	out := make([]any, 0, len(base)+len(add))
	seen := make(map[string]struct{}, len(base)+len(add))
	appendSlice := func(in []any) {
		for i := range in {
			s, _ := in[i].(string)
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	appendSlice(base)
	appendSlice(add)
	return out
}

func snapshotForAPIAccess(permissions []any, mc jwt.MapClaims) ([]byte, error) {
	m := map[string]any{"omit_roles": true}
	if len(permissions) > 0 {
		m["permissions"] = permissions
	}
	if mc != nil {
		if s, _ := mc["idp_iss"].(string); strings.TrimSpace(s) != "" {
			m["idp_iss"] = strings.TrimSpace(s)
		}
		if s, _ := mc["idp_sub"].(string); strings.TrimSpace(s) != "" {
			m["idp_sub"] = strings.TrimSpace(s)
		}
	}
	return json.Marshal(m)
}
