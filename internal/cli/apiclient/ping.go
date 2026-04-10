package apiclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ping performs GET {serverURL}/health and expects HTTP 200.
func Ping(ctx context.Context, client *http.Client, serverURL string) error {
	if client == nil {
		client = http.DefaultClient
	}
	u := strings.TrimRight(serverURL, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// PingDefaultTimeout wraps Ping with a 10s deadline if ctx has none.
func PingDefaultTimeout(ctx context.Context, client *http.Client, serverURL string) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	return Ping(ctx, client, serverURL)
}
