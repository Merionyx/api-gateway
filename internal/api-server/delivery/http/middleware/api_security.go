package middleware

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

// CtxKeyAPIJWTClaims is the Fiber Locals key for validated API-profile JWT claims (snake_case MapClaims).
const CtxKeyAPIJWTClaims = "merionyx.auth.api_jwt_claims"

// APISecurity enforces OpenAPI security for routes that are not explicitly public (roadmap ш. 20).
// Accepts either a valid API-profile Bearer JWT or a known X-API-Key (SHA-256 digest lookup; principal in Locals, roadmap ш. 21).
func APISecurity(jwtUC *auth.JWTUseCase, apiKeys APIKeyRecordGetter) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() == http.MethodOptions {
			return c.Next()
		}
		path := c.Path()
		if !requiresAPISecurity(c.Method(), path) {
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

func requiresAPISecurity(method, path string) bool {
	switch path {
	case "/health", "/ready", "/api/v1/version",
		"/.well-known/jwks.json", "/.well-known/jwks-edge.json",
		"/api/v1/keys",
		"/api/v1/auth/login", "/api/v1/auth/callback":
		return false
	case "/api/v1/auth/refresh":
		return method != http.MethodPost
	default:
		return true
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
