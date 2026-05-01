package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func entraExtraRoles(e *config.EntraOIDCProviderConfig, mc jwt.MapClaims) ([]string, error) {
	if e == nil {
		e = &config.EntraOIDCProviderConfig{}
	}
	allowedTenants := trimStringSliceNonEmpty(e.AllowedTenantIDs)
	allowedGroups := trimStringSliceNonEmpty(e.AllowedIDTokenGroups)
	needTenant := len(allowedTenants) > 0
	needGate := len(allowedGroups) > 0
	needBind := len(e.GroupRoleBindings) > 0
	if !needTenant && !needGate && !needBind {
		return nil, nil
	}

	tid := strings.TrimSpace(googleStringClaim(mc, "tid"))
	if needTenant {
		if tid == "" {
			return nil, apierrors.ErrEntraLoginDenied
		}
		tidLower := strings.ToLower(tid)
		matched := false
		for _, a := range allowedTenants {
			if strings.ToLower(strings.TrimSpace(a)) == tidLower {
				matched = true
				break
			}
		}
		if !matched {
			return nil, apierrors.ErrEntraLoginDenied
		}
	}

	groups := idTokenStringArrayClaim(mc, "groups")
	if needGate {
		if len(groups) == 0 {
			return nil, apierrors.ErrEntraLoginDenied
		}
		if !idTokenGroupsIntersectAllowed(groups, allowedGroups) {
			return nil, apierrors.ErrEntraLoginDenied
		}
	}
	if !needBind {
		return nil, nil
	}
	set := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		set[g] = struct{}{}
	}
	var extras []string
	for _, b := range e.GroupRoleBindings {
		gn := strings.TrimSpace(b.Group)
		if gn == "" {
			continue
		}
		if _, ok := set[gn]; !ok {
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
