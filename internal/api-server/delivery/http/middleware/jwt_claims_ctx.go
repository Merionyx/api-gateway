package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

// APIJWTClaimsFromCtx returns validated API-profile Bearer claims set by APISecurity.
func APIJWTClaimsFromCtx(c fiber.Ctx) (jwt.MapClaims, bool) {
	v, ok := c.Locals(CtxKeyAPIJWTClaims).(jwt.MapClaims)
	return v, ok && v != nil
}
