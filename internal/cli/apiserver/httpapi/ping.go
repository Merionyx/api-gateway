package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Ping performs GET /health (liveness) and succeeds only on HTTP 200 with a parsed health body.
func Ping(ctx context.Context, httpClient *http.Client, serverURL string) error {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return err
	}
	resp, err := c.GetHealthWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return nil
	}
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// PingDefaultTimeout wraps Ping with a 10s deadline if ctx has none.
func PingDefaultTimeout(ctx context.Context, httpClient *http.Client, serverURL string) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	return Ping(ctx, httpClient, serverURL)
}
