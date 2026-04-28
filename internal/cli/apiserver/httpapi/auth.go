package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

type RequestedTokenTTLs struct {
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type SessionTokens struct {
	AccessToken      string
	RefreshToken     string
	TokenType        string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// RefreshSession exchanges a saved refresh token for a rotated access/refresh pair.
func RefreshSession(ctx context.Context, httpClient *http.Client, serverURL, refreshToken string, requestedTTLs RequestedTokenTTLs) (*SessionTokens, error) {
	ctx, cancel := withServerTimeout(ctx)
	defer cancel()

	rt := strings.TrimSpace(refreshToken)
	if rt == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}

	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	resp, err := c.TokenOidcWithFormdataBodyWithResponse(ctx, apiserverclient.TokenOidcFormdataRequestBody{
		GrantType:    apiserverclient.RefreshToken,
		RefreshToken: &rt,
		AccessTtl:    optionalSeconds(requestedTTLs.AccessTTL),
		RefreshTtl:   optionalSeconds(requestedTTLs.RefreshTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		tokenType := strings.TrimSpace(resp.JSON200.Data.TokenType)
		if tokenType == "" {
			tokenType = "Bearer"
		}
		refreshTokenOut := ""
		if resp.JSON200.Data.RefreshToken != nil {
			refreshTokenOut = strings.TrimSpace(*resp.JSON200.Data.RefreshToken)
		}
		if refreshTokenOut == "" {
			return nil, fmt.Errorf("api: refresh_token is missing in token response")
		}
		accessExpiresAt := time.Now().UTC().Add(time.Duration(resp.JSON200.Data.ExpiresIn) * time.Second)
		if resp.JSON200.Data.AccessExpiresAt != nil {
			accessExpiresAt = resp.JSON200.Data.AccessExpiresAt.UTC()
		}
		refreshExpiresAt := accessExpiresAt
		if resp.JSON200.Data.RefreshExpiresAt != nil {
			refreshExpiresAt = resp.JSON200.Data.RefreshExpiresAt.UTC()
		} else if resp.JSON200.Data.RefreshExpiresIn != nil && *resp.JSON200.Data.RefreshExpiresIn > 0 {
			refreshExpiresAt = time.Now().UTC().Add(time.Duration(*resp.JSON200.Data.RefreshExpiresIn) * time.Second)
		}
		return &SessionTokens{
			AccessToken:      resp.JSON200.Data.AccessToken,
			RefreshToken:     refreshTokenOut,
			TokenType:        tokenType,
			AccessExpiresAt:  accessExpiresAt,
			RefreshExpiresAt: refreshExpiresAt,
		}, nil
	}
	if resp.JSON400 != nil {
		return nil, fmt.Errorf("api: %s", oauthTokenErrorString(resp.JSON400))
	}
	if resp.JSON503 != nil {
		return nil, fmt.Errorf("api: %s", oauthTokenErrorString(resp.JSON503))
	}
	if resp.JSON500 != nil {
		return nil, fmt.Errorf("api: %s", oauthTokenErrorString(resp.JSON500))
	}
	return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func oauthTokenErrorString(e *apiserverclient.OAuthTokenError) string {
	if e == nil {
		return ""
	}
	msg := strings.TrimSpace(e.Error)
	if e.ErrorDescription != nil && strings.TrimSpace(*e.ErrorDescription) != "" {
		if msg != "" {
			return msg + ": " + strings.TrimSpace(*e.ErrorDescription)
		}
		return strings.TrimSpace(*e.ErrorDescription)
	}
	return msg
}

func optionalSeconds(d time.Duration) *int {
	if d <= 0 {
		return nil
	}
	v := int(d / time.Second)
	return &v
}
