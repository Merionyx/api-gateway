package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/merionyx/api-gateway/internal/api-server/config"
)

// claimsSnapshotFromProvider builds the JSON claims_snapshot stored on the session after OIDC
// callback and refresh (IdP-up path) using CEL-based mapping rules.
func claimsSnapshotFromProvider(ctx context.Context, p config.OIDCProviderConfig, idClaims jwt.MapClaims, idpOAuthAccess string, hc *http.Client) (json.RawMessage, error) {
	mapped, err := applyOIDCClaimMapping(ctx, p, idClaims, idpOAuthAccess, hc)
	if err != nil {
		return nil, err
	}
	return marshalClaimsSnapshot(mapped.Claims, mapped.Roles, mapped.Permissions)
}

func mergeUniqueStrings(base, add []string) []string {
	seen := make(map[string]struct{}, len(base)+len(add))
	var out []string
	for _, s := range base {
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
	for _, s := range add {
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
	return out
}

func marshalClaimsSnapshot(claims map[string]any, roles, permissions []string) (json.RawMessage, error) {
	m := make(map[string]any)
	for k, v := range claims {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		m[kk] = normalizeClaimValue(v)
	}
	m["roles"] = roles
	if len(permissions) > 0 {
		m["permissions"] = permissions
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}
