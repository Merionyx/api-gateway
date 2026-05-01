package authz

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
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
