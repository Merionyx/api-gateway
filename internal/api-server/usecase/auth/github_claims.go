package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/githubapi"
	"github.com/merionyx/api-gateway/internal/api-server/auth/gitlabapi"
	apiroles "github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// githubRESTBaseURL is overridden in tests (empty = production api.github.com).
var githubRESTBaseURL = ""

// gitlabRESTBaseURL is overridden in tests (empty = derive from issuer).
var gitlabRESTBaseURL = ""

func githubRESTBaseFor(gh *config.GitHubOIDCProviderConfig) string {
	if gh != nil {
		if b := strings.TrimSpace(gh.RESTAPIBase); b != "" {
			return b
		}
	}
	return githubRESTBaseURL
}

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
	return marshalClaimsSnapshot(idClaims, roles)
}

func googleExtraRoles(g *config.GoogleOIDCProviderConfig, mc jwt.MapClaims) ([]string, error) {
	if g == nil {
		g = &config.GoogleOIDCProviderConfig{}
	}
	allowedHD := stringSetLowerTrim(g.AllowedHostedDomains)
	allowedEmailDom := stringSetLowerTrim(g.AllowedEmailDomains)
	needHD := len(allowedHD) > 0
	needEmail := len(allowedEmailDom) > 0
	needBindHD := len(g.HostedDomainRoleBindings) > 0
	needBindEmail := len(g.EmailDomainRoleBindings) > 0
	if !needHD && !needEmail && !needBindHD && !needBindEmail {
		return nil, nil
	}

	email := strings.TrimSpace(googleStringClaim(mc, "email"))
	hd := strings.TrimSpace(googleStringClaim(mc, "hd"))

	if needHD {
		if hd == "" {
			return nil, apierrors.ErrGoogleLoginDenied
		}
		if _, ok := allowedHD[strings.ToLower(hd)]; !ok {
			return nil, apierrors.ErrGoogleLoginDenied
		}
	} else if needEmail {
		d := emailDomainFromAddress(email)
		if d == "" {
			return nil, apierrors.ErrGoogleLoginDenied
		}
		if _, ok := allowedEmailDom[d]; !ok {
			return nil, apierrors.ErrGoogleLoginDenied
		}
	}

	var extras []string
	for _, b := range g.HostedDomainRoleBindings {
		bhd := strings.TrimSpace(b.HD)
		if bhd == "" {
			continue
		}
		if strings.EqualFold(hd, bhd) {
			for _, r := range b.Roles {
				if s := strings.TrimSpace(r); s != "" {
					extras = append(extras, s)
				}
			}
		}
	}
	dom := emailDomainFromAddress(email)
	for _, b := range g.EmailDomainRoleBindings {
		d := strings.TrimSpace(strings.ToLower(b.Domain))
		if d == "" {
			continue
		}
		if dom == d {
			for _, r := range b.Roles {
				if s := strings.TrimSpace(r); s != "" {
					extras = append(extras, s)
				}
			}
		}
	}
	return extras, nil
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

func gitlabRESTBaseFor(gl *config.GitLabOIDCProviderConfig, issuer string) (string, error) {
	if gl != nil {
		if b := strings.TrimSpace(gl.APIV4Base); b != "" {
			return strings.TrimSuffix(b, "/"), nil
		}
	}
	if b := strings.TrimSpace(gitlabRESTBaseURL); b != "" {
		return strings.TrimSuffix(b, "/"), nil
	}
	return config.GitLabAPIV4BaseFromIssuer(issuer)
}

func gitlabExtraRoles(ctx context.Context, hc *http.Client, gl *config.GitLabOIDCProviderConfig, issuer, oauthAccess string) ([]string, error) {
	if gl == nil {
		gl = &config.GitLabOIDCProviderConfig{}
	}
	allowed := normalizeGitLabAllowedPaths(gl.AllowedGroupPaths)
	needGate := len(allowed) > 0
	needBindings := len(gl.GroupRoleBindings) > 0
	if !needGate && !needBindings {
		return nil, nil
	}
	oauthAccess = strings.TrimSpace(oauthAccess)
	if oauthAccess == "" {
		return nil, fmt.Errorf("%w: missing GitLab OAuth access token for group checks", apierrors.ErrInvalidInput)
	}
	apiBase, err := gitlabRESTBaseFor(gl, issuer)
	if err != nil {
		return nil, err
	}
	paths, err := gitlabapi.ListMembershipGroupFullPaths(ctx, hc, oauthAccess, apiBase)
	if err != nil {
		return nil, err
	}
	if needGate && !userMatchesAnyAllowedGitLabPath(paths, allowed) {
		return nil, apierrors.ErrGitLabLoginDenied
	}
	if !needBindings {
		return nil, nil
	}
	pathSet := make(map[string]struct{}, len(paths))
	for _, pth := range paths {
		pathSet[pth] = struct{}{}
	}
	var extras []string
	for _, b := range gl.GroupRoleBindings {
		key := strings.TrimSpace(b.GroupFullPath)
		if _, ok := pathSet[key]; !ok {
			continue
		}
		for _, r := range b.Roles {
			if s := strings.TrimSpace(r); s != "" {
				extras = append(extras, s)
			}
		}
	}
	return extras, nil
}

func normalizeGitLabPathSegment(p string) string {
	return strings.Trim(strings.TrimSpace(p), "/")
}

func normalizeGitLabAllowedPaths(in []string) []string {
	var out []string
	for _, s := range in {
		s = normalizeGitLabPathSegment(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func userMatchesAllowedGitLabPath(userPaths []string, allowedRoot string) bool {
	A := normalizeGitLabPathSegment(allowedRoot)
	if A == "" {
		return false
	}
	for _, raw := range userPaths {
		U := normalizeGitLabPathSegment(raw)
		if U == A || strings.HasPrefix(U, A+"/") {
			return true
		}
	}
	return false
}

func userMatchesAnyAllowedGitLabPath(userPaths []string, allowed []string) bool {
	for _, a := range allowed {
		if userMatchesAllowedGitLabPath(userPaths, a) {
			return true
		}
	}
	return false
}

func githubExtraRoles(ctx context.Context, hc *http.Client, gh *config.GitHubOIDCProviderConfig, oauthAccess, restBase string) ([]string, error) {
	if gh == nil {
		gh = &config.GitHubOIDCProviderConfig{}
	}

	needOrgs := len(normalizeOrgSet(gh.AllowedOrgLogins)) > 0
	needTeams := len(gh.TeamRoleBindings) > 0
	if !needOrgs && !needTeams {
		return nil, nil
	}

	oauthAccess = strings.TrimSpace(oauthAccess)
	if oauthAccess == "" {
		return nil, fmt.Errorf("%w: missing GitHub OAuth access token for org/team checks", apierrors.ErrInvalidInput)
	}

	allowed := normalizeOrgSet(gh.AllowedOrgLogins)
	if needOrgs {
		orgs, err := githubapi.ListUserOrgLogins(ctx, hc, oauthAccess, restBase)
		if err != nil {
			return nil, err
		}
		if len(allowed) > 0 && !intersectsOrg(orgs, allowed) {
			return nil, apierrors.ErrGitHubLoginDenied
		}
	}

	if !needTeams {
		return nil, nil
	}

	teams, err := githubapi.ListUserTeams(ctx, hc, oauthAccess, restBase)
	if err != nil {
		return nil, err
	}

	teamSet := make(map[string]struct{}, len(teams))
	for _, tr := range teams {
		key := strings.ToLower(tr.OrgLogin) + "/" + strings.ToLower(tr.Slug)
		teamSet[key] = struct{}{}
	}

	var extras []string
	for _, b := range gh.TeamRoleBindings {
		key := strings.ToLower(strings.TrimSpace(b.Org)) + "/" + strings.ToLower(strings.TrimSpace(b.TeamSlug))
		if _, ok := teamSet[key]; !ok {
			continue
		}
		for _, r := range b.Roles {
			if s := strings.TrimSpace(r); s != "" {
				extras = append(extras, s)
			}
		}
	}
	return extras, nil
}

func normalizeOrgSet(in []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, o := range in {
		s := strings.TrimSpace(o)
		if s == "" {
			continue
		}
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

func intersectsOrg(userOrgs []string, allowed map[string]struct{}) bool {
	for _, o := range userOrgs {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(o))]; ok {
			return true
		}
	}
	return false
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
	for _, k := range []string{"sub", "email", "name", "preferred_username", "hd"} {
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
