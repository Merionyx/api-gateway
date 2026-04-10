package handler

import (
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/api-server/usecase"

	"github.com/gofiber/fiber/v3"
)

type JWTHandler struct {
	jwtUseCase     *usecase.JWTUseCase
	metricsEnabled bool
}

func NewJWTHandler(jwtUseCase *usecase.JWTUseCase, metricsEnabled bool) *JWTHandler {
	return &JWTHandler{jwtUseCase: jwtUseCase, metricsEnabled: metricsEnabled}
}

// GenerateToken generates a JWT token
// POST /api/v1/tokens
func (h *JWTHandler) GenerateToken(c fiber.Ctx) error {
	var req models.GenerateTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationBind)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validation
	if req.AppID == "" {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationAppID)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "app_id is required",
		})
	}

	if len(req.Environments) == 0 {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationEnvironments)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "environments are required",
		})
	}

	for _, environment := range req.Environments {
		if environment == "" {
			apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationEmptyEnv)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "environment is required",
			})
		}
	}

	if req.ExpiresAt.Before(time.Now()) {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationExpiresAt)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "expires_at must be in the future",
		})
	}

	token, err := h.jwtUseCase.GenerateToken(&req)
	if err != nil {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultInternalError)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultCreated)
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
