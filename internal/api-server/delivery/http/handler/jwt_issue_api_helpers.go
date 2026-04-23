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

func rolesFromAPIJWTClaims(mc jwt.MapClaims) []any {
	v, ok := mc["roles"]
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
	default:
		return []any{}
	}
}

func rolesStringsToAny(in []string) []any {
	if len(in) == 0 {
		return []any{}
	}
	out := make([]any, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}

func snapshotForAPIAccess(roles []any, mc jwt.MapClaims) ([]byte, error) {
	m := map[string]any{"roles": roles}
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
