package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	apiroles "github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/config"
)

// claimsSnapshotFromProvider builds the JSON claims_snapshot stored on the session after OIDC
// callback and refresh (IdP-up path). Provider-specific enrichment adds roles on top of api:member.
func claimsSnapshotFromProvider(ctx context.Context, p config.OIDCProviderConfig, idClaims jwt.MapClaims, idpOAuthAccess string, hc *http.Client) (json.RawMessage, error) {
	roles := []string{apiroles.APIMember}
	if p.IsGitHubOIDCProvider() {
		extras, err := githubExtraRoles(ctx, hc, p.GitHub, idpOAuthAccess, githubRESTBaseFor(p.GitHub))
		if err != nil {
			return nil, err
		}
		roles = mergeUniqueStrings(roles, extras)
	}
	if p.IsGitLabOIDCProvider() {
		extras, err := gitlabExtraRoles(ctx, hc, p.GitLab, strings.TrimSpace(p.Issuer), idpOAuthAccess)
		if err != nil {
			return nil, err
		}
		roles = mergeUniqueStrings(roles, extras)
	}
	if p.IsGoogleOIDCProvider() {
		extras, err := googleExtraRoles(p.Google, idClaims)
		if err != nil {
			return nil, err
		}
		roles = mergeUniqueStrings(roles, extras)
	}
	if p.IsOktaOIDCProvider() {
		extras, err := oktaExtraRoles(p.Okta, idClaims)
		if err != nil {
			return nil, err
		}
		roles = mergeUniqueStrings(roles, extras)
	}
	if p.IsEntraOIDCProvider() {
		extras, err := entraExtraRoles(p.Entra, idClaims)
		if err != nil {
			return nil, err
		}
		roles = mergeUniqueStrings(roles, extras)
	}
	return marshalClaimsSnapshot(idClaims, roles)
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

func marshalClaimsSnapshot(mc jwt.MapClaims, roles []string) (json.RawMessage, error) {
	m := map[string]any{
		"roles": roles,
	}
	for _, k := range []string{"sub", "email", "name", "preferred_username", "hd", "tid"} {
		if v, ok := mc[k]; ok {
			m[k] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}
