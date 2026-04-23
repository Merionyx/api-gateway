package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/auth/githubapi"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// githubRESTBaseURL is overridden in tests (empty = production api.github.com).
var githubRESTBaseURL = ""

func githubRESTBaseFor(gh *config.GitHubOIDCProviderConfig) string {
	if gh != nil {
		if b := strings.TrimSpace(gh.RESTAPIBase); b != "" {
			return b
		}
	}
	return githubRESTBaseURL
}

func githubExtraRoles(ctx context.Context, hc *http.Client, gh *config.GitHubOIDCProviderConfig, oauthAccess, restBase string) ([]string, error) {
	if gh == nil {
		gh = &config.GitHubOIDCProviderConfig{}
	}

	needOrgs := len(normalizeOrgSet(gh.AllowedOrgLogins)) > 0
	needTeams := len(gh.TeamRoleBindings) > 0
	if !needOrgs && !needTeams {
		return nil, nil
	}

	oauthAccess = strings.TrimSpace(oauthAccess)
	if oauthAccess == "" {
		return nil, fmt.Errorf("%w: missing GitHub OAuth access token for org/team checks", apierrors.ErrInvalidInput)
	}

	allowed := normalizeOrgSet(gh.AllowedOrgLogins)
	if needOrgs {
		orgs, err := githubapi.ListUserOrgLogins(ctx, hc, oauthAccess, restBase)
		if err != nil {
			return nil, err
		}
		if len(allowed) > 0 && !intersectsOrg(orgs, allowed) {
			return nil, apierrors.ErrGitHubLoginDenied
		}
	}

	if !needTeams {
		return nil, nil
	}

	teams, err := githubapi.ListUserTeams(ctx, hc, oauthAccess, restBase)
	if err != nil {
		return nil, err
	}

	teamSet := make(map[string]struct{}, len(teams))
	for _, tr := range teams {
		key := strings.ToLower(tr.OrgLogin) + "/" + strings.ToLower(tr.Slug)
		teamSet[key] = struct{}{}
	}

	var extras []string
	for _, b := range gh.TeamRoleBindings {
		key := strings.ToLower(strings.TrimSpace(b.Org)) + "/" + strings.ToLower(strings.TrimSpace(b.TeamSlug))
		if _, ok := teamSet[key]; !ok {
			continue
		}
		for _, r := range b.Roles {
			if s := strings.TrimSpace(r); s != "" {
				extras = append(extras, s)
			}
		}
	}
	return extras, nil
}

func normalizeOrgSet(in []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, o := range in {
		s := strings.TrimSpace(o)
		if s == "" {
			continue
		}
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

func intersectsOrg(userOrgs []string, allowed map[string]struct{}) bool {
	for _, o := range userOrgs {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(o))]; ok {
			return true
		}
	}
	return false
}
