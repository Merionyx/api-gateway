package authz

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
)

// NormalizeRolesValue turns JWT / JSON `roles` shapes into non-empty trimmed strings.
func NormalizeRolesValue(v any) []string {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			s, _ := e.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// SubjectRoles returns role strings for the current request (Bearer JWT first, then API key).
func SubjectRoles(c fiber.Ctx) []string {
	if mc, ok := middleware.APIJWTClaimsFromCtx(c); ok {
		return NormalizeRolesValue(mc["roles"])
	}
	if p, ok := middleware.APIKeyPrincipalFromCtx(c); ok {
		out := make([]string, 0, len(p.Roles))
		for _, s := range p.Roles {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func subjectHasAdmin(have []string) bool {
	for _, s := range have {
		if s == roles.APIRoleAdmin {
			return true
		}
	}
	return false
}

func subjectHasAny(have []string, required []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, s := range have {
		set[s] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}

// RequireAnyHTTPRole writes **403** and returns **true** when the caller lacks required roles.
// It returns **false** when the caller has **api:role:admin**, any of **required**, or when **required** is empty.
// Unauthenticated callers must be rejected earlier by APISecurity (problem.Write typically returns nil on success,
// so a bool is required — do not use the returned error alone as a guard).
func RequireAnyHTTPRole(c fiber.Ctx, required ...string) (denied bool, _ error) {
	if len(required) == 0 {
		return false, nil
	}
	have := SubjectRoles(c)
	if subjectHasAdmin(have) {
		return false, nil
	}
	if len(have) == 0 {
		err := problem.Write(c, http.StatusForbidden, problem.Forbidden(
			"INSUFFICIENT_ROLES",
			"",
			"The caller has no roles for this operation. Configure API key roles or identity mapping (CEL).",
		))
		return true, err
	}
	if !subjectHasAny(have, required) {
		err := problem.Write(c, http.StatusForbidden, problem.Forbidden(
			"INSUFFICIENT_ROLES",
			"",
			"The caller does not have any of the roles required for this operation.",
		))
		return true, err
	}
	return false, nil
}
