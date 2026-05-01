package authz

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
)

// PermissionEvaluator resolves subject permissions from roles plus token/API-key direct permissions.
type PermissionEvaluator struct {
	roleCatalog *roles.Catalog
}

// NewPermissionEvaluator builds a permission evaluator.
func NewPermissionEvaluator(roleCatalog *roles.Catalog) *PermissionEvaluator {
	if roleCatalog == nil {
		roleCatalog, _ = roles.NewCatalog(nil)
	}
	return &PermissionEvaluator{
		roleCatalog: roleCatalog,
	}
}

// RequireAnyHTTPPermission writes 403 and returns true when caller lacks required permissions.
func (e *PermissionEvaluator) RequireAnyHTTPPermission(c fiber.Ctx, required ...string) (denied bool, _ error) {
	required = normalizeStrings(required)
	if len(required) == 0 {
		return false, nil
	}
	have, err := e.SubjectPermissions(c)
	if err != nil {
		return true, problem.WriteInternal(c, err)
	}
	if len(have) == 0 {
		err := problem.Write(c, http.StatusForbidden, problem.Forbidden(
			problem.CodeInsufficientPermissions,
			"",
			"The caller has no permissions for this operation. Configure role->permissions or token grants.",
		))
		return true, err
	}
	for i := range required {
		if HasPermission(have, required[i]) {
			return false, nil
		}
	}
	err = problem.Write(c, http.StatusForbidden, problem.Forbidden(
		problem.CodeInsufficientPermissions,
		"",
		"The caller does not have any required permission for this operation.",
	))
	return true, err
}

// RequireDelegatedPermissions verifies caller is allowed to request all delegated permissions.
func (e *PermissionEvaluator) RequireDelegatedPermissions(c fiber.Ctx, delegated []string) (denied bool, _ error) {
	delegated = normalizeStrings(delegated)
	if len(delegated) == 0 {
		return false, nil
	}
	have, err := e.SubjectPermissions(c)
	if err != nil {
		return true, problem.WriteInternal(c, err)
	}
	if HasPermission(have, permissions.Wildcard) {
		return false, nil
	}
	for i := range delegated {
		if HasPermission(have, delegated[i]) {
			continue
		}
		err := problem.Write(c, http.StatusForbidden, problem.Forbidden(
			problem.CodeRequestedPermissionsNotAllowed,
			"",
			"The caller cannot delegate one or more requested permissions.",
		))
		return true, err
	}
	return false, nil
}

// SubjectPermissions returns effective permissions for current request subject.
func (e *PermissionEvaluator) SubjectPermissions(c fiber.Ctx) (map[string]struct{}, error) {
	if e == nil {
		return map[string]struct{}{}, nil
	}
	out := e.roleCatalog.ResolvePermissions(SubjectRoles(c))

	if p, ok := middleware.APIKeyPrincipalFromCtx(c); ok {
		for _, s := range normalizeStrings(p.Scopes) {
			out[s] = struct{}{}
		}
		return out, nil
	}

	mc, ok := middleware.APIJWTClaimsFromCtx(c)
	if !ok {
		return out, nil
	}
	for _, s := range normalizeStrings(ClaimStrings(mc, "permissions")) {
		out[s] = struct{}{}
	}
	for _, s := range normalizeStrings(ClaimStrings(mc, "scopes")) {
		out[s] = struct{}{}
	}
	return out, nil
}

// HasPermission checks exact permission membership, honoring wildcard grants.
func HasPermission(have map[string]struct{}, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return false
	}
	if _, ok := have[permissions.Wildcard]; ok {
		return true
	}
	_, ok := have[required]
	return ok
}

// ClaimStrings extracts a string/slice claim as a normalized list of unique non-empty values.
func ClaimStrings(mc jwt.MapClaims, key string) []string {
	v, ok := mc[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return normalizeStrings(x)
	case []any:
		out := make([]string, 0, len(x))
		for i := range x {
			s, _ := x[i].(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return normalizeStrings(out)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func normalizeStrings(in ...[]string) []string {
	if len(in) == 0 {
		return nil
	}
	var n int
	for i := range in {
		n += len(in[i])
	}
	out := make([]string, 0, n)
	seen := make(map[string]struct{}, n)
	for i := range in {
		for j := range in[i] {
			s := strings.TrimSpace(in[i][j])
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
	return out
}
