// Package githubapi calls GitHub REST for interactive OIDC follow-up (orgs, teams).
package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DefaultRESTBase is the production GitHub API origin.
const DefaultRESTBase = "https://api.github.com"

// UserAgent is sent on every request (GitHub requires a non-empty User-Agent).
const UserAgent = "api-gateway-control-plane/oidc (+https://github.com/merionyx/api-gateway)"

// TeamRef identifies a GitHub team the authenticated user belongs to (from GET /user/teams).
type TeamRef struct {
	OrgLogin string
	Slug     string
}

type orgWire struct {
	Login string `json:"login"`
}

type teamWire struct {
	Slug         string `json:"slug"`
	Organization struct {
		Login string `json:"login"`
	} `json:"organization"`
}

// UserInfo is the subset of GitHub user profile used to build interactive claims without id_token.
type UserInfo struct {
	ID    int64
	Login string
	Name  string
	Email string
}

type userWire struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func restBase(base string) string {
	b := strings.TrimSuffix(strings.TrimSpace(base), "/")
	if b == "" {
		return DefaultRESTBase
	}
	return b
}

// ListUserOrgLogins returns organization logins for the OAuth user (GET /user/orgs, paginated).
func ListUserOrgLogins(ctx context.Context, hc *http.Client, oauthToken, restBaseURL string) ([]string, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	tok := strings.TrimSpace(oauthToken)
	if tok == "" {
		return nil, fmt.Errorf("githubapi: empty oauth token")
	}
	base := restBase(restBaseURL)
	var out []string
	page := 1
	for {
		u, err := url.Parse(base + "/user/orgs")
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprintf("%d", page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/vnd.github+json")
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
			return nil, fmt.Errorf("githubapi: list orgs: status %d: %s", resp.StatusCode, truncate(body, 256))
		}
		var batch []orgWire
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("githubapi: list orgs json: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, o := range batch {
			if s := strings.TrimSpace(o.Login); s != "" {
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

// ListUserTeams returns teams visible to the OAuth user (GET /user/teams, paginated).
func ListUserTeams(ctx context.Context, hc *http.Client, oauthToken, restBaseURL string) ([]TeamRef, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	tok := strings.TrimSpace(oauthToken)
	if tok == "" {
		return nil, fmt.Errorf("githubapi: empty oauth token")
	}
	base := restBase(restBaseURL)
	var out []TeamRef
	page := 1
	for {
		u, err := url.Parse(base + "/user/teams")
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprintf("%d", page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/vnd.github+json")
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
			return nil, fmt.Errorf("githubapi: list teams: status %d: %s", resp.StatusCode, truncate(body, 256))
		}
		var batch []teamWire
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("githubapi: list teams json: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, t := range batch {
			org := strings.TrimSpace(t.Organization.Login)
			slug := strings.TrimSpace(t.Slug)
			if org != "" && slug != "" {
				out = append(out, TeamRef{OrgLogin: org, Slug: slug})
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

// GetAuthenticatedUser returns the OAuth user profile from GET /user.
func GetAuthenticatedUser(ctx context.Context, hc *http.Client, oauthToken, restBaseURL string) (UserInfo, error) {
	var out UserInfo
	if hc == nil {
		hc = http.DefaultClient
	}
	tok := strings.TrimSpace(oauthToken)
	if tok == "" {
		return out, fmt.Errorf("githubapi: empty oauth token")
	}
	base := restBase(restBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/user", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := hc.Do(req)
	if err != nil {
		return out, err
	}
	body, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if rerr != nil {
		return out, rerr
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("githubapi: get user: status %d: %s", resp.StatusCode, truncate(body, 256))
	}
	var uw userWire
	if err := json.Unmarshal(body, &uw); err != nil {
		return out, fmt.Errorf("githubapi: get user json: %w", err)
	}
	out = UserInfo{
		ID:    uw.ID,
		Login: strings.TrimSpace(uw.Login),
		Name:  strings.TrimSpace(uw.Name),
		Email: strings.TrimSpace(uw.Email),
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
