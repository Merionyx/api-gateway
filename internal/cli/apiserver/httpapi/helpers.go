package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

func newClientWithResponses(serverURL string, httpClient *http.Client) (*apiserverclient.ClientWithResponses, error) {
	base := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return apiserverclient.NewClientWithResponses(base, apiserverclient.WithHTTPClient(httpClient))
}

// problemString renders RFC 7807 Problem for CLI / operator messages (prefers code + detail).
func problemString(p *apiserverclient.Problem) string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	if p.Code != nil && *p.Code != "" {
		_, _ = b.WriteString(*p.Code)
	}
	if p.Detail != nil && strings.TrimSpace(*p.Detail) != "" {
		if b.Len() > 0 {
			_, _ = b.WriteString(": ")
		}
		_, _ = b.WriteString(strings.TrimSpace(*p.Detail))
	} else if strings.TrimSpace(p.Title) != "" {
		if b.Len() > 0 {
			_, _ = b.WriteString(": ")
		}
		_, _ = b.WriteString(strings.TrimSpace(p.Title))
	}
	if b.Len() == 0 {
		return fmt.Sprintf("HTTP %d", p.Status)
	}
	return b.String()
}

func trimBody(body []byte) string {
	const max = 2048
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
