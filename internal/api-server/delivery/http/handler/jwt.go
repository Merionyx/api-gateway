package handler

import (
	"net/http"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	"github.com/gofiber/fiber/v3"
)

const defaultAPIAccessTokenTTL = 5 * time.Minute

// JWTHandler serves JWT/JWKS HTTP endpoints (roadmap ш. 15, 22).
type JWTHandler struct {
	jwtUseCase     *auth.JWTUseCase
	metricsEnabled bool
	apiAccessTTL   time.Duration
}

// NewJWTHandler wires JWT HTTP handlers. apiAccessTTL<=0 defaults to 5m (POST /api/v1/tokens/api).
func NewJWTHandler(jwtUseCase *auth.JWTUseCase, metricsEnabled bool, apiAccessTTL time.Duration) *JWTHandler {
	if apiAccessTTL <= 0 {
		apiAccessTTL = defaultAPIAccessTokenTTL
	}
	return &JWTHandler{
		jwtUseCase:     jwtUseCase,
		metricsEnabled: metricsEnabled,
		apiAccessTTL:   apiAccessTTL,
	}
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

// IssueApiAccessToken mints a short-lived API-profile JWT (POST /api/v1/tokens/api; roadmap ш. 22).
// Caller must already be authenticated (API-profile Bearer and/or X-API-Key via APISecurity).
func (h *JWTHandler) IssueApiAccessToken(c fiber.Ctx) error {
	span := beginHandlerSpan(c, "IssueApiAccessToken")
	defer span.End()

	p, pOK := middleware.APIKeyPrincipalFromCtx(c)
	mc, jOK := middleware.APIJWTClaimsFromCtx(c)
	if !pOK && !jOK {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultInternalError)
		return problem.Write(c, http.StatusUnauthorized, problem.Unauthorized(
			"AUTH_CONTEXT_MISSING",
			"",
			"Authenticated context is required to issue API access tokens.",
		))
	}

	if denied, werr := authz.RequireAnyHTTPRole(c, roles.APIAccessTokensIssue); denied {
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultForbidden)
		return werr
	}

	if len(c.Body()) > 0 {
		var body apiserver.IssueApiAccessTokenRequest
		if err := c.Bind().Body(&body); err != nil {
			telemetry.MarkError(span, err)
			apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationBind)
			return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
		}
		_ = body.RequestedScopes
	}

	var subject string
	var snap []byte
	var err error
	if pOK {
		subject = "m2m:" + p.DigestHex
		snap, err = snapshotForAPIAccess(rolesStringsToAny(p.Roles), nil)
	} else {
		subject = subjectFromAPIJWTClaims(mc)
		if subject == "" {
			apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultValidationAppID)
			return problem.Write(c, http.StatusBadRequest, problem.BadRequest("API_TOKEN_SUBJECT_MISSING", "", "Bearer token has no usable sub/email for API access issuance."))
		}
		snap, err = snapshotForAPIAccess(rolesFromAPIJWTClaims(mc), mc)
	}
	if err != nil {
		telemetry.MarkError(span, err)
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultInternalError)
		return problem.WriteInternal(c, err)
	}

	token, _, exp, err := h.jwtUseCase.MintInteractiveAPIAccessJWTFromSnapshot(c.Context(), subject, snap, h.apiAccessTTL)
	if err != nil {
		telemetry.MarkError(span, err)
		apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultInternalError)
		return problem.RespondError(c, err)
	}

	apimetrics.RecordTokenGenerate(h.metricsEnabled, apimetrics.TokenResultCreated)
	out := apiserver.ApiAccessTokenIssued{
		AccessToken: token,
		ExpiresAt:   exp,
	}
	return c.Status(fiber.StatusCreated).JSON(out)
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

// GetJWKSEdge returns the Edge-profile JSON Web Key Set (GET /.well-known/jwks-edge.json).
func (h *JWTHandler) GetJWKSEdge(c fiber.Ctx) error {
	span := beginHandlerSpan(c, "GetJWKSEdge")
	defer span.End()
	jwks, err := h.jwtUseCase.GetJWKSEdge(c.Context())
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
