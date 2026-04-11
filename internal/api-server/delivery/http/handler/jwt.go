package handler

import (
	"net/http"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"

	"github.com/gofiber/fiber/v3"
)

type JWTHandler struct {
	jwtUseCase     *auth.JWTUseCase
	metricsEnabled bool
}

func NewJWTHandler(jwtUseCase *auth.JWTUseCase, metricsEnabled bool) *JWTHandler {
	return &JWTHandler{jwtUseCase: jwtUseCase, metricsEnabled: metricsEnabled}
}

// GenerateToken generates a JWT token
// POST /api/v1/tokens
func (h *JWTHandler) GenerateToken(c fiber.Ctx) error {
	var req models.GenerateTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationBind)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest("", "invalid request body"))
	}

	if req.AppID == "" {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationAppID)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest("", "app_id is required"))
	}

	if len(req.Environments) == 0 {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationEnvironments)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest("", "environments are required"))
	}

	for _, environment := range req.Environments {
		if environment == "" {
			apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationEmptyEnv)
			return problem.Write(c, http.StatusBadRequest, problem.BadRequest("", "environment is required"))
		}
	}

	if req.ExpiresAt.Before(time.Now()) {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationExpiresAt)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest("", "expires_at must be in the future"))
	}

	token, err := h.jwtUseCase.GenerateToken(&req)
	if err != nil {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultInternalError)
		return problem.RespondError(c, err)
	}

	apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultCreated)
	return c.Status(fiber.StatusCreated).JSON(token)
}

// GetJWKS returns a JSON Web Key Set
// GET /.well-known/jwks.json
func (h *JWTHandler) GetJWKS(c fiber.Ctx) error {
	jwks, err := h.jwtUseCase.GetJWKS()
	if err != nil {
		return problem.RespondError(c, err)
	}

	return c.JSON(jwks)
}

// GetSigningKeys returns a list of signing keys
// GET /api/v1/keys
func (h *JWTHandler) GetSigningKeys(c fiber.Ctx) error {
	keys := h.jwtUseCase.GetSigningKeys()
	return c.JSON(keys)
}
