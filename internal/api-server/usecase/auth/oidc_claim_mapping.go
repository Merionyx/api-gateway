package auth

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/cel-go/cel"

	"github.com/merionyx/api-gateway/internal/api-server/auth/githubapi"
	"github.com/merionyx/api-gateway/internal/api-server/auth/gitlabapi"
	apiroles "github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

type oidcClaimMappingDenyError struct {
	RuleName string
	Code     string
	Message  string
}

func (e *oidcClaimMappingDenyError) Error() string {
	if e == nil {
		return ""
	}
	detail := strings.TrimSpace(e.Message)
	if detail != "" {
		return fmt.Sprintf("%v: %s", apierrors.ErrOIDCClaimMappingDenied, detail)
	}
	if s := strings.TrimSpace(e.RuleName); s != "" {
		return fmt.Sprintf("%v: rule %q", apierrors.ErrOIDCClaimMappingDenied, s)
	}
	return apierrors.ErrOIDCClaimMappingDenied.Error()
}

func (e *oidcClaimMappingDenyError) Unwrap() error {
	return apierrors.ErrOIDCClaimMappingDenied
}

type oidcClaimMappingResult struct {
	Roles       []string
	Permissions []string
	Groups      []string
	Claims      map[string]any
}

func applyOIDCClaimMapping(ctx context.Context, p config.OIDCProviderConfig, idClaims jwt.MapClaims, idpOAuthAccess string, hc *http.Client) (oidcClaimMappingResult, error) {
	baseClaims := defaultSnapshotClaims(idClaims)
	res := oidcClaimMappingResult{
		Roles:       []string{apiroles.APIRoleViewer},
		Permissions: nil,
		Groups:      nil,
		Claims:      baseClaims,
	}
	mapping := p.ClaimMapping
	if mapping == nil || len(mapping.Rules) == 0 {
		return res, nil
	}

	input, err := buildOIDCClaimMappingInput(ctx, p, idClaims, strings.TrimSpace(idpOAuthAccess), hc)
	if err != nil {
		return oidcClaimMappingResult{}, err
	}

	env, err := cel.NewEnv(
		cel.Variable("provider", cel.DynType),
		cel.Variable("id_token", cel.DynType),
		cel.Variable("auth", cel.DynType),
	)
	if err != nil {
		return oidcClaimMappingResult{}, fmt.Errorf("%w: cel env: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
	}

	vars := map[string]any{
		"provider": input["provider"],
		"id_token": input["id_token"],
		"auth":     input["auth"],
	}

	for i := range mapping.Rules {
		rule := mapping.Rules[i]
		matched, err := evalCELBool(env, rule.When, vars)
		if err != nil {
			name := strings.TrimSpace(rule.Name)
			if name == "" {
				name = fmt.Sprintf("index=%d", i)
			}
			return oidcClaimMappingResult{}, fmt.Errorf("%w: rule %q when: %w", apierrors.ErrOIDCClaimMappingRuntime, name, err)
		}
		if !matched {
			continue
		}

		if rule.Deny {
			return oidcClaimMappingResult{}, &oidcClaimMappingDenyError{
				RuleName: strings.TrimSpace(rule.Name),
				Code:     strings.TrimSpace(rule.DenyCode),
				Message:  strings.TrimSpace(rule.DenyMessage),
			}
		}

		res.Roles = mergeUniqueStrings(res.Roles, rule.AddRoles)
		res.Permissions = mergeUniqueStrings(res.Permissions, rule.AddPermissions)
		res.Groups = mergeUniqueStrings(res.Groups, rule.AddGroups)

		if s := strings.TrimSpace(rule.AddRolesExpr); s != "" {
			add, err := evalCELStringList(env, s, vars)
			if err != nil {
				return oidcClaimMappingResult{}, fmt.Errorf("%w: add_roles_expr: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
			}
			res.Roles = mergeUniqueStrings(res.Roles, add)
		}
		if s := strings.TrimSpace(rule.AddPermissionsExpr); s != "" {
			add, err := evalCELStringList(env, s, vars)
			if err != nil {
				return oidcClaimMappingResult{}, fmt.Errorf("%w: add_permissions_expr: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
			}
			res.Permissions = mergeUniqueStrings(res.Permissions, add)
		}
		if s := strings.TrimSpace(rule.AddGroupsExpr); s != "" {
			add, err := evalCELStringList(env, s, vars)
			if err != nil {
				return oidcClaimMappingResult{}, fmt.Errorf("%w: add_groups_expr: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
			}
			res.Groups = mergeUniqueStrings(res.Groups, add)
		}

		if len(rule.SetClaims) > 0 {
			keys := make([]string, 0, len(rule.SetClaims))
			for k := range rule.SetClaims {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, key := range keys {
				expr := strings.TrimSpace(rule.SetClaims[key])
				v, err := evalCELAny(env, expr, vars)
				if err != nil {
					return oidcClaimMappingResult{}, fmt.Errorf("%w: set_claims[%q]: %w", apierrors.ErrOIDCClaimMappingRuntime, key, err)
				}
				res.Claims[strings.TrimSpace(key)] = normalizeClaimValue(v)
			}
		}
	}

	if len(res.Groups) > 0 {
		res.Claims["groups"] = append([]string(nil), res.Groups...)
	}
	if len(res.Permissions) > 0 {
		res.Claims["permissions"] = append([]string(nil), res.Permissions...)
	}
	if len(res.Roles) == 0 {
		res.Roles = []string{apiroles.APIRoleViewer}
	}
	return res, nil
}

func defaultSnapshotClaims(mc jwt.MapClaims) map[string]any {
	m := make(map[string]any)
	for _, k := range []string{"sub", "email", "name", "preferred_username", "hd", "tid", "idp_iss", "idp_sub"} {
		if v, ok := mc[k]; ok {
			m[k] = normalizeClaimValue(v)
		}
	}
	return m
}

func buildOIDCClaimMappingInput(ctx context.Context, p config.OIDCProviderConfig, idClaims jwt.MapClaims, idpOAuthAccess string, hc *http.Client) (map[string]any, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	provider := map[string]any{
		"id":     strings.TrimSpace(p.ID),
		"name":   strings.TrimSpace(p.Name),
		"kind":   strings.ToLower(strings.TrimSpace(p.Kind)),
		"issuer": strings.TrimSpace(p.Issuer),
	}
	if provider["kind"] == "" {
		provider["kind"] = "generic"
	}

	if p.IsGitHubOIDCProvider() {
		gh := map[string]any{
			"org_logins": []string{},
			"teams":      []map[string]any{},
		}
		needOrgs := claimMappingNeeds(p.ClaimMapping, "provider.github.org_logins")
		needTeams := claimMappingNeeds(p.ClaimMapping, "provider.github.teams")
		if idpOAuthAccess != "" && (needOrgs || needTeams) {
			orgs, err := githubapi.ListUserOrgLogins(ctx, hc, idpOAuthAccess, githubRESTBaseFor(p.GitHub))
			if err != nil {
				return nil, fmt.Errorf("%w: github orgs: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
			}
			gh["org_logins"] = orgs
			if needTeams {
				teams, err := githubapi.ListUserTeams(ctx, hc, idpOAuthAccess, githubRESTBaseFor(p.GitHub))
				if err != nil {
					return nil, fmt.Errorf("%w: github teams: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
				}
				teamRows := make([]map[string]any, 0, len(teams))
				for i := range teams {
					teamRows = append(teamRows, map[string]any{
						"org_login": strings.TrimSpace(teams[i].OrgLogin),
						"slug":      strings.TrimSpace(teams[i].Slug),
					})
				}
				gh["teams"] = teamRows
			}
		}
		provider["github"] = gh
	}

	if p.IsGitLabOIDCProvider() {
		gl := map[string]any{
			"group_full_paths": []string{},
		}
		needGroups := claimMappingNeeds(p.ClaimMapping, "provider.gitlab.group_full_paths")
		if idpOAuthAccess != "" && needGroups {
			apiBase, err := gitlabRESTBaseFor(p.GitLab, strings.TrimSpace(p.Issuer))
			if err != nil {
				return nil, fmt.Errorf("%w: gitlab api base: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
			}
			groups, err := gitlabapi.ListMembershipGroupFullPaths(ctx, hc, idpOAuthAccess, apiBase)
			if err != nil {
				return nil, fmt.Errorf("%w: gitlab groups: %w", apierrors.ErrOIDCClaimMappingRuntime, err)
			}
			gl["group_full_paths"] = groups
		}
		provider["gitlab"] = gl
	}

	idToken := make(map[string]any, len(idClaims))
	for k, v := range idClaims {
		idToken[k] = normalizeClaimValue(v)
	}

	return map[string]any{
		"provider": provider,
		"id_token": idToken,
		"auth": map[string]any{
			"has_provider_access_token": idpOAuthAccess != "",
		},
	}, nil
}

func claimMappingNeeds(m *config.OIDCClaimMappingConfig, needle string) bool {
	if m == nil || len(m.Rules) == 0 {
		return false
	}
	for i := range m.Rules {
		r := m.Rules[i]
		if strings.Contains(r.When, needle) ||
			strings.Contains(r.AddRolesExpr, needle) ||
			strings.Contains(r.AddPermissionsExpr, needle) ||
			strings.Contains(r.AddGroupsExpr, needle) {
			return true
		}
		for _, expr := range r.SetClaims {
			if strings.Contains(expr, needle) {
				return true
			}
		}
	}
	return false
}

func evalCELBool(env *cel.Env, expr string, vars map[string]any) (bool, error) {
	v, err := evalCELAny(env, expr, vars)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expression must return bool, got %T", v)
	}
	return b, nil
}

func evalCELStringList(env *cel.Env, expr string, vars map[string]any) ([]string, error) {
	v, err := evalCELAny(env, expr, vars)
	if err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	case []string:
		return trimStringSliceNonEmpty(t), nil
	case []any:
		out := make([]string, 0, len(t))
		for i := range t {
			s, ok := t[i].(string)
			if !ok {
				return nil, fmt.Errorf("expected string list item, got %T", t[i])
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expression must return string or list(string), got %T", v)
	}
}

func evalCELAny(env *cel.Env, expr string, vars map[string]any) (any, error) {
	ast, issues := env.Compile(strings.TrimSpace(expr))
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, err
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, err
	}
	native, err := out.ConvertToNative(reflect.TypeOf((*any)(nil)).Elem())
	if err != nil {
		return nil, err
	}
	return native, nil
}

func normalizeClaimValue(v any) any {
	switch t := v.(type) {
	case []string:
		out := make([]any, 0, len(t))
		for i := range t {
			s := strings.TrimSpace(t[i])
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for i := range t {
			out = append(out, normalizeClaimValue(t[i]))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			out[k] = normalizeClaimValue(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			out[fmt.Sprint(k)] = normalizeClaimValue(vv)
		}
		return out
	case float64, float32, int, int32, int64, uint, uint32, uint64, bool, nil:
		return t
	case string:
		return strings.TrimSpace(t)
	default:
		return fmt.Sprint(t)
	}
}
