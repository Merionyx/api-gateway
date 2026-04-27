package handler

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

// OAuthTokenHandler serves POST /api/v1/auth/callback as OAuth 2.1 token endpoint.
type OAuthTokenHandler struct {
	uc *auth.OAuthTokenUseCase
}

func NewOAuthTokenHandler(uc *auth.OAuthTokenUseCase) *OAuthTokenHandler {
	return &OAuthTokenHandler{uc: uc}
}

func (h *OAuthTokenHandler) Token(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 35*time.Second)
	defer cancel()

	grantType := strings.TrimSpace(c.FormValue("grant_type"))
	if grantType == "" {
		grantType = strings.TrimSpace(c.Query("grant_type"))
	}
	clientID := strings.TrimSpace(c.FormValue("client_id"))
	if clientID == "" {
		clientID = clientIDFromBasicAuth(c.Get(fiber.HeaderAuthorization))
	}

	req := auth.OAuthTokenRequest{
		GrantType:           grantType,
		Code:                strings.TrimSpace(c.FormValue("code")),
		RedirectURI:         strings.TrimSpace(c.FormValue("redirect_uri")),
		ClientID:            clientID,
		CodeVerifier:        strings.TrimSpace(c.FormValue("code_verifier")),
		RefreshToken:        strings.TrimSpace(c.FormValue("refresh_token")),
		RequestedAccessTTL:  durationFromOptionalFormSeconds(c.FormValue("requested_access_token_ttl_seconds")),
		RequestedRefreshTTL: durationFromOptionalFormSeconds(c.FormValue("requested_refresh_token_ttl_seconds")),
	}

	out, err := h.uc.Exchange(ctx, req)
	if err != nil {
		status, oauthErr, description := auth.MapOAuthTokenError(err)
		return c.Status(status).JSON(fiber.Map{
			"error":             oauthErr,
			"error_description": description,
		})
	}
	return c.JSON(out)
}

func durationFromOptionalFormSeconds(v string) time.Duration {
	s := strings.TrimSpace(v)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

func clientIDFromBasicAuth(h string) string {
	raw := strings.TrimSpace(h)
	if len(raw) < len("Basic ") || !strings.EqualFold(raw[:len("Basic ")], "Basic ") {
		return ""
	}
	payload := strings.TrimSpace(raw[len("Basic "):])
	b, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
