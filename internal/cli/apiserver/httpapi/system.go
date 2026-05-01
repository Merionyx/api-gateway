package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

const serverHTTPTimeout = 30 * time.Second

func withServerTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, serverHTTPTimeout)
}

// Ready calls GET /ready. Returns parsed body for HTTP 200 or 503.
func Ready(ctx context.Context, httpClient *http.Client, serverURL string) (*apiserverclient.ReadinessStatus, int, error) {
	ctx, cancel := withServerTimeout(ctx)
	defer cancel()
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.GetReadyWithResponse(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return &resp.JSON200.Data, http.StatusOK, nil
	}
	if resp.JSON503 != nil {
		return resp.JSON503, http.StatusServiceUnavailable, nil
	}
	if resp.ApplicationproblemJSON400 != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	return nil, resp.StatusCode(), fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// ServerVersion calls GET /v1/version.
func ServerVersion(ctx context.Context, httpClient *http.Client, serverURL string) (*apiserverclient.VersionResponse, error) {
	ctx, cancel := withServerTimeout(ctx)
	defer cancel()
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	resp, err := c.GetVersionWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return &resp.JSON200.Data, nil
	}
	if resp.ApplicationproblemJSON500 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// ServerJWKS calls GET /.well-known/jwks.json.
func ServerJWKS(ctx context.Context, httpClient *http.Client, serverURL string) (*apiserverclient.Jwks, error) {
	ctx, cancel := withServerTimeout(ctx)
	defer cancel()
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	params := &apiserverclient.GetJwksParams{}
	resp, err := c.GetJwksWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	if resp.ApplicationproblemJSON400 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// ServerStatus calls GET /v1/status.
func ServerStatus(ctx context.Context, httpClient *http.Client, serverURL string) (*apiserverclient.StatusResponse, error) {
	ctx, cancel := withServerTimeout(ctx)
	defer cancel()
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	params := &apiserverclient.GetStatusParams{}
	resp, err := c.GetStatusWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return &resp.JSON200.Data, nil
	}
	if resp.ApplicationproblemJSON400 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return nil, fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}
