package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func trimStringSliceNonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// idTokenStringArrayClaim reads a string or JSON array-of-strings claim (e.g. Okta/Entra "groups").
func idTokenStringArrayClaim(mc jwt.MapClaims, key string) []string {
	raw, ok := mc[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		return []string{s}
	case []string:
		var out []string
		for _, s := range v {
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
		return out
	case []any:
		var out []string
		for _, x := range v {
			s, ok := x.(string)
			if !ok {
				continue
			}
			if t := strings.TrimSpace(s); t != "" {
				out = append(out, t)
			}
		}
		return out
	default:
		return nil
	}
}

func idTokenGroupsIntersectAllowed(userGroups, allowed []string) bool {
	allowSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowSet[a] = struct{}{}
	}
	for _, u := range userGroups {
		if _, ok := allowSet[u]; ok {
			return true
		}
	}
	return false
}

func googleStringClaim(mc jwt.MapClaims, k string) string {
	v, ok := mc[k]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func stringSetLowerTrim(in []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

func emailDomainFromAddress(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:]))
}
