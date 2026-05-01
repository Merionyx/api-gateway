// Package gitlabapi calls GitLab REST API v4 for OIDC follow-up (group membership).
package gitlabapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// UserAgent is sent on every request.
const UserAgent = "api-gateway-control-plane/oidc (+https://github.com/merionyx/api-gateway)"

type groupWire struct {
	FullPath string `json:"full_path"`
}

// ListMembershipGroupFullPaths returns full_path for each group the OAuth user is a member of
// (GET /api/v4/groups?membership=true, paginated). apiV4BaseURL must be the API root, e.g. https://gitlab.com/api/v4.
func ListMembershipGroupFullPaths(ctx context.Context, hc *http.Client, oauthToken, apiV4BaseURL string) ([]string, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	tok := strings.TrimSpace(oauthToken)
	if tok == "" {
		return nil, fmt.Errorf("gitlabapi: empty oauth token")
	}
	root := strings.TrimSuffix(strings.TrimSpace(apiV4BaseURL), "/")
	if root == "" {
		return nil, fmt.Errorf("gitlabapi: empty api v4 base url")
	}

	var out []string
	page := 1
	for {
		u, err := url.Parse(root + "/groups")
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("membership", "true")
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprintf("%d", page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", UserAgent)

		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if rerr != nil {
			return nil, rerr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("gitlabapi: list groups: status %d: %s", resp.StatusCode, truncate(body, 256))
		}
		var batch []groupWire
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("gitlabapi: list groups json: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, g := range batch {
			if s := strings.TrimSpace(g.FullPath); s != "" {
				out = append(out, s)
			}
		}
		if len(batch) < 100 {
			break
		}
		page++
		if page > 50 {
			break
		}
	}
	return out, nil
}

func truncate(b []byte, max int) string {
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
