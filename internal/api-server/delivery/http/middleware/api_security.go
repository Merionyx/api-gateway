package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

// CtxKeyAPIJWTClaims is the Fiber Locals key for validated API-profile JWT claims (snake_case MapClaims).
const CtxKeyAPIJWTClaims = "merionyx.auth.api_jwt_claims"

// APISecurityContract is a method+path matcher for OpenAPI operations explicitly declared as public (security: []).
type APISecurityContract struct {
	publicByMethod map[string][]openAPIPathPattern
}

type openAPIPathPattern struct {
	segments []string
}

func NewAPISecurityContract(swagger *openapi3.T) (*APISecurityContract, error) {
	if swagger == nil {
		return nil, errors.New("nil OpenAPI document")
	}
	if swagger.Paths == nil {
		return nil, errors.New("OpenAPI document has no paths")
	}

	contract := &APISecurityContract{publicByMethod: make(map[string][]openAPIPathPattern)}
	for specPath, pathItem := range swagger.Paths.Map() {
		if pathItem == nil {
			continue
		}
		for method, op := range pathItem.Operations() {
			if op == nil || op.Security == nil || len(*op.Security) != 0 {
				continue
			}
			m := strings.ToUpper(strings.TrimSpace(method))
			if m == "" {
				continue
			}
			contract.publicByMethod[m] = append(contract.publicByMethod[m], newOpenAPIPathPattern(specPath))
		}
	}

	return contract, nil
}

func (c *APISecurityContract) IsPublic(method, path string) bool {
	if c == nil {
		return false
	}
	publicPaths := c.publicByMethod[strings.ToUpper(strings.TrimSpace(method))]
	if len(publicPaths) == 0 {
		return false
	}

	requestSegments := pathSegments(path)
	for _, p := range publicPaths {
		if p.matches(requestSegments) {
			return true
		}
	}
	return false
}

func (c *APISecurityContract) RequiresAPISecurity(method, path string) bool {
	return !c.IsPublic(method, path)
}

func newOpenAPIPathPattern(specPath string) openAPIPathPattern {
	return openAPIPathPattern{segments: pathSegments(specPath)}
}

func (p openAPIPathPattern) matches(requestSegments []string) bool {
	if len(p.segments) != len(requestSegments) {
		return false
	}
	for i, seg := range p.segments {
		if isPathTemplateSegment(seg) {
			if requestSegments[i] == "" {
				return false
			}
			continue
		}
		if seg != requestSegments[i] {
			return false
		}
	}
	return true
}

func isPathTemplateSegment(segment string) bool {
	return len(segment) >= 3 && strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func pathSegments(raw string) []string {
	trimmed := strings.Trim(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

// APISecurity enforces OpenAPI security for routes that are not explicitly public .
// Accepts either a valid API-profile Bearer JWT or a known X-API-Key (SHA-256 digest lookup; principal in Locals).
func APISecurity(jwtUC *auth.JWTUseCase, apiKeys APIKeyRecordGetter, contract *APISecurityContract) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() == http.MethodOptions {
			return c.Next()
		}
		if contract != nil && !contract.RequiresAPISecurity(c.Method(), c.Path()) {
			return c.Next()
		}

		if jwtUC != nil {
			if raw := parseBearer(c.Get(fiber.HeaderAuthorization)); raw != "" {
				if mc, err := jwtUC.ParseAndValidateAPIProfileBearerToken(raw); err == nil {
					c.Locals(CtxKeyAPIJWTClaims, mc)
					return c.Next()
				}
			}
		}

		if apiKeys != nil {
			if p, err := tryAPIKeyPrincipal(c.Context(), apiKeys, c.Get("X-API-Key")); err != nil {
				return problem.WriteInternal(c, err)
			} else if p != nil {
				c.Locals(CtxKeyAPIKeyPrincipal, p)
				return c.Next()
			}
		}

		return problem.Write(c, http.StatusUnauthorized, problem.Unauthorized(
			"AUTHENTICATION_REQUIRED",
			"",
			"Send Authorization: Bearer with an API-profile JWT (see /.well-known/jwks.json), or a valid X-API-Key.",
		))
	}
}

func parseBearer(h string) string {
	h = strings.TrimSpace(h)
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
