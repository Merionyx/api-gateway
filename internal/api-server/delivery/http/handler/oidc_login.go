package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/safelog"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

// OIDCLoginHandler serves GET /api/v1/auth/authorize (roadmap ш. 13).
type OIDCLoginHandler struct {
	uc *auth.OIDCLoginUseCase
}

// NewOIDCLoginHandler constructs the handler.
func NewOIDCLoginHandler(uc *auth.OIDCLoginUseCase) *OIDCLoginHandler {
	return &OIDCLoginHandler{uc: uc}
}

// Authorize starts OAuth2/OIDC authorization code + PKCE and responds with 302 to the IdP.
func (h *OIDCLoginHandler) Authorize(c fiber.Ctx, params apiserver.AuthorizeOidcParams) error {
	ctx, cancel := context.WithTimeout(c.Context(), 25*time.Second)
	defer cancel()

	nonce := ""
	if params.Nonce != nil {
		nonce = *params.Nonce
	}

	loc, err := h.uc.Start(ctx, auth.OIDCLoginStartRequest{
		ProviderID:          stringOrEmpty(params.ProviderId),
		RedirectURI:         params.RedirectUri,
		ServerCallbackURI:   c.BaseURL() + "/api/v1/auth/callback",
		Nonce:               nonce,
		RequestedAccessTTL:  durationFromOptionalSeconds(params.RequestedAccessTokenTtlSeconds),
		RequestedRefreshTTL: durationFromOptionalSeconds(params.RequestedRefreshTokenTtlSeconds),
		ResponseType:        string(params.ResponseType),
		ClientID:            params.ClientId,
		State:               stringOrEmpty(params.State),
		CodeChallenge:       params.CodeChallenge,
		CodeChallengeMethod: string(params.CodeChallengeMethod),
	})
	if err != nil {
		st, code, detail := auth.MapStartError(err)
		switch st {
		case http.StatusBadRequest:
			return problem.Write(c, st, problem.WithCode(st, code, "", detail))
		case http.StatusBadGateway:
			slog.Error("oidc login discovery", "err", safelog.Redact(err.Error()))
			return problem.Write(c, st, problem.BadGateway(code, "", detail))
		case http.StatusServiceUnavailable:
			slog.Error("oidc login store", "err", safelog.Redact(err.Error()))
			return problem.Write(c, st, problem.ServiceUnavailable(code, "", detail))
		default:
			return problem.WriteInternal(c, err)
		}
	}

	c.Response().Header.Set("Location", loc)
	c.Status(http.StatusFound)
	return nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ListOidcProviders returns public metadata for configured browser OIDC providers (GET /api/v1/auth/oidc-providers).
func (h *OIDCLoginHandler) ListOidcProviders(c fiber.Ctx) error {
	rows := h.uc.ListPublicOIDCProviders()
	out := make([]apiserver.OidcProviderDescriptor, len(rows))
	for i, r := range rows {
		out[i] = apiserver.OidcProviderDescriptor{Id: r.ID, Name: r.Name, Kind: r.Kind, Issuer: r.Issuer}
	}
	return c.JSON(out)
}
