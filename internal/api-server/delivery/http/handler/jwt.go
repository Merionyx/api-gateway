package handler

import (
	"net/http"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

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
// POST /api/v1/tokens/edge (Edge profile; OpenAPI operation issueEdgeToken).
func (h *JWTHandler) GenerateToken(c fiber.Ctx) error {
	span := beginHandlerSpan(c, "GenerateToken")
	defer span.End()
	var req models.GenerateTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		telemetry.MarkError(span, err)
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationBind)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
	}

	if req.AppID == "" {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationAppID)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeTokenAppIDRequired, "", problem.DetailTokenAppIDRequired))
	}

	if len(req.Environments) == 0 {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationEnvironments)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeTokenEnvironmentsRequired, "", problem.DetailTokenEnvironmentsRequired))
	}

	for _, environment := range req.Environments {
		if environment == "" {
			apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationEmptyEnv)
			return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeTokenEnvironmentEmpty, "", problem.DetailTokenEnvironmentEmpty))
		}
	}

	if req.ExpiresAt.Before(time.Now()) {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationExpiresAt)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeTokenExpiresAtPast, "", problem.DetailTokenExpiresAtPast))
	}

	token, err := h.jwtUseCase.GenerateToken(c.Context(), &req)
	if err != nil {
		telemetry.MarkError(span, err)
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultInternalError)
		return problem.RespondError(c, err)
	}

	apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultCreated)
	return c.Status(fiber.StatusCreated).JSON(token)
}

// GetJWKS returns a JSON Web Key Set
// GET /.well-known/jwks.json
func (h *JWTHandler) GetJWKS(c fiber.Ctx) error {
	span := beginHandlerSpan(c, "GetJWKS")
	defer span.End()
	jwks, err := h.jwtUseCase.GetJWKS(c.Context())
	if err != nil {
		telemetry.MarkError(span, err)
		return problem.RespondError(c, err)
	}

	return c.JSON(jwks)
}

// GetSigningKeys returns a list of signing keys
// GET /api/v1/keys
func (h *JWTHandler) GetSigningKeys(c fiber.Ctx) error {
	span := beginHandlerSpan(c, "GetSigningKeys")
	defer span.End()
	keys := h.jwtUseCase.GetSigningKeys(c.Context())
	return c.JSON(keys)
}
