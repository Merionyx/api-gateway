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

// OIDCCallbackHandler serves GET /api/v1/auth/callback (roadmap ш. 14).
type OIDCCallbackHandler struct {
	uc *auth.OIDCCallbackUseCase
}

// NewOIDCCallbackHandler constructs the handler.
func NewOIDCCallbackHandler(uc *auth.OIDCCallbackUseCase) *OIDCCallbackHandler {
	return &OIDCCallbackHandler{uc: uc}
}

// Callback completes the authorization code flow and returns our token pair as JSON.
func (h *OIDCCallbackHandler) Callback(c fiber.Ctx, params apiserver.CallbackOidcParams) error {
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	out, err := h.uc.Complete(ctx, params.Code, params.State)
	if err != nil {
		st, code, detail := auth.MapCallbackError(err)
		switch st {
		case http.StatusBadRequest:
			return problem.Write(c, st, problem.WithCode(st, code, "", detail))
		case http.StatusUnauthorized:
			return problem.Write(c, st, problem.WithCode(st, code, "", detail))
		case http.StatusBadGateway:
			slog.Error("oidc callback discovery", "err", err)
			return problem.Write(c, st, problem.BadGateway(code, "", detail))
		case http.StatusServiceUnavailable:
			slog.Error("oidc callback dependency", "err", err)
			return problem.Write(c, st, problem.ServiceUnavailable(code, "", detail))
		default:
			return problem.WriteInternal(c, err)
		}
	}

	return c.JSON(out)
}
