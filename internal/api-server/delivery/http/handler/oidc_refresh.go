package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

// OIDCRefreshHandler serves POST /api/v1/auth/refresh (roadmap ш. 17 IdP up).
type OIDCRefreshHandler struct {
	uc *auth.OIDCRefreshUseCase
}

// NewOIDCRefreshHandler constructs the handler.
func NewOIDCRefreshHandler(uc *auth.OIDCRefreshUseCase) *OIDCRefreshHandler {
	return &OIDCRefreshHandler{uc: uc}
}

// Refresh exchanges our refresh_token for a new access/refresh pair (IdP up branch).
func (h *OIDCRefreshHandler) Refresh(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 35*time.Second)
	defer cancel()

	var body apiserver.AuthRefreshRequest
	if err := c.Bind().Body(&body); err != nil {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
	}
	if body.RefreshToken == "" {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest("REFRESH_TOKEN_REQUIRED", "", "refresh_token is required."))
	}

	out, err := h.uc.Refresh(ctx, body.RefreshToken)
	if err != nil {
		st, code, detail := auth.MapRefreshError(err)
		switch st {
		case http.StatusBadRequest:
			return problem.Write(c, st, problem.WithCode(st, code, "", detail))
		case http.StatusUnauthorized:
			return problem.Write(c, st, problem.WithCode(st, code, "", detail))
		case http.StatusConflict:
			return problem.Write(c, st, problem.Conflict(code, "", detail))
		case http.StatusBadGateway:
			slog.Error("oidc refresh discovery", "err", err)
			return problem.Write(c, st, problem.BadGateway(code, "", detail))
		case http.StatusServiceUnavailable:
			slog.Error("oidc refresh dependency", "err", err)
			return problem.Write(c, st, problem.ServiceUnavailable(code, "", detail))
		default:
			return problem.WriteInternal(c, err)
		}
	}

	return c.JSON(out)
}
