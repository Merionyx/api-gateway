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

// RefreshSession exchanges a saved refresh token for a rotated access/refresh pair.
func RefreshSession(ctx context.Context, httpClient *http.Client, serverURL, refreshToken string, requestedTTLs RequestedTokenTTLs) (*apiserverclient.AuthSessionTokensResponse, error) {
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
	resp, err := c.RefreshSessionWithResponse(ctx, apiserverclient.AuthRefreshRequest{
		RefreshToken:                    rt,
		RequestedAccessTokenTtlSeconds:  optionalSeconds(requestedTTLs.AccessTTL),
		RequestedRefreshTokenTtlSeconds: optionalSeconds(requestedTTLs.RefreshTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	if resp.ApplicationproblemJSON400 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON401 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON401))
	}
	if resp.ApplicationproblemJSON409 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON409))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func optionalSeconds(d time.Duration) *int {
	if d <= 0 {
		return nil
	}
	v := int(d / time.Second)
	return &v
}
