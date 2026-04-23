package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/auth/gitlabapi"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// gitlabRESTBaseURL is overridden in tests (empty = derive from issuer).
var gitlabRESTBaseURL = ""

func gitlabRESTBaseFor(gl *config.GitLabOIDCProviderConfig, issuer string) (string, error) {
	if gl != nil {
		if b := strings.TrimSpace(gl.APIV4Base); b != "" {
			return strings.TrimSuffix(b, "/"), nil
		}
	}
	if b := strings.TrimSpace(gitlabRESTBaseURL); b != "" {
		return strings.TrimSuffix(b, "/"), nil
	}
	return config.GitLabAPIV4BaseFromIssuer(issuer)
}

func gitlabExtraRoles(ctx context.Context, hc *http.Client, gl *config.GitLabOIDCProviderConfig, issuer, oauthAccess string) ([]string, error) {
	if gl == nil {
		gl = &config.GitLabOIDCProviderConfig{}
	}
	allowed := normalizeGitLabAllowedPaths(gl.AllowedGroupPaths)
	needGate := len(allowed) > 0
	needBindings := len(gl.GroupRoleBindings) > 0
	if !needGate && !needBindings {
		return nil, nil
	}
	oauthAccess = strings.TrimSpace(oauthAccess)
	if oauthAccess == "" {
		return nil, fmt.Errorf("%w: missing GitLab OAuth access token for group checks", apierrors.ErrInvalidInput)
	}
	apiBase, err := gitlabRESTBaseFor(gl, issuer)
	if err != nil {
		return nil, err
	}
	paths, err := gitlabapi.ListMembershipGroupFullPaths(ctx, hc, oauthAccess, apiBase)
	if err != nil {
		return nil, err
	}
	if needGate && !userMatchesAnyAllowedGitLabPath(paths, allowed) {
		return nil, apierrors.ErrGitLabLoginDenied
	}
	if !needBindings {
		return nil, nil
	}
	pathSet := make(map[string]struct{}, len(paths))
	for _, pth := range paths {
		pathSet[pth] = struct{}{}
	}
	var extras []string
	for _, b := range gl.GroupRoleBindings {
		key := strings.TrimSpace(b.GroupFullPath)
		if _, ok := pathSet[key]; !ok {
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

func normalizeGitLabPathSegment(p string) string {
	return strings.Trim(strings.TrimSpace(p), "/")
}

func normalizeGitLabAllowedPaths(in []string) []string {
	var out []string
	for _, s := range in {
		s = normalizeGitLabPathSegment(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func userMatchesAllowedGitLabPath(userPaths []string, allowedRoot string) bool {
	A := normalizeGitLabPathSegment(allowedRoot)
	if A == "" {
		return false
	}
	for _, raw := range userPaths {
		U := normalizeGitLabPathSegment(raw)
		if U == A || strings.HasPrefix(U, A+"/") {
			return true
		}
	}
	return false
}

func userMatchesAnyAllowedGitLabPath(userPaths []string, allowed []string) bool {
	for _, a := range allowed {
		if userMatchesAllowedGitLabPath(userPaths, a) {
			return true
		}
	}
	return false
}
