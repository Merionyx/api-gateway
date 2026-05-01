package openapi

import (
	"math"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func subjectFromAPIJWTClaims(mc jwt.MapClaims) string {
	if e, _ := mc["email"].(string); strings.TrimSpace(e) != "" {
		return strings.TrimSpace(e)
	}
	if p, _ := mc["preferred_username"].(string); strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	if s, _ := mc["sub"].(string); strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return ""
}

func permissionsFromAPIJWTClaims(mc jwt.MapClaims) []any {
	return mergeAnyUnique(claimSliceToAny(mc, "permissions"), claimSliceToAny(mc, "scopes"))
}

func hasAnyRoleClaim(mc jwt.MapClaims) bool {
	return len(claimSliceToAny(mc, "roles")) > 0
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

func claimString(mc jwt.MapClaims, key string) string {
	v, ok := mc[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

func numericUnixClaimToTime(mc jwt.MapClaims, key string) (time.Time, bool) {
	v, ok := mc[key]
	if !ok || v == nil {
		return time.Time{}, false
	}
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return time.Time{}, false
		}
		return time.Unix(int64(x), 0).UTC(), true
	case int64:
		return time.Unix(x, 0).UTC(), true
	case int:
		return time.Unix(int64(x), 0).UTC(), true
	default:
		return time.Time{}, false
	}
}
