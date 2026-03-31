package handler

import (
	"time"

	"merionyx/api-gateway/internal/api-server/domain/models"
	"merionyx/api-gateway/internal/api-server/usecase"

	"github.com/gofiber/fiber/v3"
)

type JWTHandler struct {
	jwtUseCase *usecase.JWTUseCase
}

func NewJWTHandler(jwtUseCase *usecase.JWTUseCase) *JWTHandler {
	return &JWTHandler{jwtUseCase: jwtUseCase}
}

// GenerateToken generates a JWT token
// POST /api/v1/tokens
func (h *JWTHandler) GenerateToken(c fiber.Ctx) error {
	var req models.GenerateTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validation
	if req.AppID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "app_id is required",
		})
	}

	if req.ExpiresAt.Before(time.Now()) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "expires_at must be in the future",
		})
	}

	token, err := h.jwtUseCase.GenerateToken(&req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(token)
}

// GetJWKS returns a JSON Web Key Set
// GET /.well-known/jwks.json
func (h *JWTHandler) GetJWKS(c fiber.Ctx) error {
	jwks, err := h.jwtUseCase.GetJWKS()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(jwks)
}

// GetSigningKeys returns a list of signing keys
// GET /api/v1/keys
func (h *JWTHandler) GetSigningKeys(c fiber.Ctx) error {
	keys := h.jwtUseCase.GetSigningKeys()
	return c.JSON(keys)
}
