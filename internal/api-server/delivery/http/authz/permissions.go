package authz

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// TokenGrantReader loads per-token delegated permissions by JWT jti.
type TokenGrantReader interface {
	Get(ctx context.Context, jti string) (kvvalue.TokenGrantValue, int64, error)
}

// PermissionEvaluator resolves subject permissions from roles + direct grants.
type PermissionEvaluator struct {
	roleCatalog *roles.Catalog
	tokenGrants TokenGrantReader
}

// NewPermissionEvaluator builds a permission evaluator.
func NewPermissionEvaluator(roleCatalog *roles.Catalog, tokenGrants TokenGrantReader) *PermissionEvaluator {
	if roleCatalog == nil {
		roleCatalog, _ = roles.NewCatalog(nil)
	}
	return &PermissionEvaluator{
		roleCatalog: roleCatalog,
		tokenGrants: tokenGrants,
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
		if hasPermission(have, required[i]) {
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
	if hasPermission(have, permissions.Wildcard) {
		return false, nil
	}
	for i := range delegated {
		if hasPermission(have, delegated[i]) {
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
	for _, s := range normalizeStrings(claimStrings(mc, "permissions")) {
		out[s] = struct{}{}
	}
	for _, s := range normalizeStrings(claimStrings(mc, "scopes")) {
		out[s] = struct{}{}
	}
	if e.tokenGrants == nil {
		return out, nil
	}
	jti := claimString(mc, "jti")
	if jti == "" {
		return out, nil
	}
	rec, _, err := e.tokenGrants.Get(c.Context(), jti)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return out, nil
		}
		return nil, err
	}
	// ExpiresAt mirrors token exp; stale records are ignored to keep auth decisions deterministic.
	if !rec.ExpiresAt.IsZero() && !time.Now().Before(rec.ExpiresAt) {
		return out, nil
	}
	for _, p := range normalizeStrings(rec.Permissions) {
		out[p] = struct{}{}
	}
	return out, nil
}

func hasPermission(have map[string]struct{}, required string) bool {
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

func claimStrings(mc jwt.MapClaims, key string) []string {
	v, ok := mc[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for i := range x {
			s, _ := x[i].(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{x}
	default:
		return nil
	}
}

func claimString(mc jwt.MapClaims, key string) string {
	s, _ := mc[key].(string)
	return strings.TrimSpace(s)
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
