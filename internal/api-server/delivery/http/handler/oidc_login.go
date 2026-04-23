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

// OIDCLoginHandler serves GET /api/v1/auth/login (roadmap ш. 13).
type OIDCLoginHandler struct {
	uc *auth.OIDCLoginUseCase
}

// NewOIDCLoginHandler constructs the handler.
func NewOIDCLoginHandler(uc *auth.OIDCLoginUseCase) *OIDCLoginHandler {
	return &OIDCLoginHandler{uc: uc}
}

// Login starts OIDC authorization code + PKCE and responds with 302 to the IdP.
func (h *OIDCLoginHandler) Login(c fiber.Ctx, params apiserver.LoginOidcParams) error {
	ctx, cancel := context.WithTimeout(c.Context(), 25*time.Second)
	defer cancel()

	nonce := ""
	if params.Nonce != nil {
		nonce = *params.Nonce
	}

	loc, err := h.uc.Start(ctx, params.ProviderId, params.RedirectUri, nonce)
	if err != nil {
		st, code, detail := auth.MapStartError(err)
		switch st {
		case http.StatusBadRequest:
			return problem.Write(c, st, problem.WithCode(st, code, "", detail))
		case http.StatusBadGateway:
			slog.Error("oidc login discovery", "err", err)
			return problem.Write(c, st, problem.BadGateway(code, "", detail))
		case http.StatusServiceUnavailable:
			slog.Error("oidc login store", "err", err)
			return problem.Write(c, st, problem.ServiceUnavailable(code, "", detail))
		default:
			return problem.WriteInternal(c, err)
		}
	}

	c.Response().Header.Set("Location", loc)
	c.Status(http.StatusFound)
	return nil
}
