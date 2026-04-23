package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func oktaExtraRoles(o *config.OktaOIDCProviderConfig, mc jwt.MapClaims) ([]string, error) {
	if o == nil {
		o = &config.OktaOIDCProviderConfig{}
	}
	allowed := trimStringSliceNonEmpty(o.AllowedIDTokenGroups)
	needGate := len(allowed) > 0
	needBind := len(o.GroupRoleBindings) > 0
	if !needGate && !needBind {
		return nil, nil
	}
	groups := idTokenStringArrayClaim(mc, "groups")
	if needGate {
		if len(groups) == 0 {
			return nil, apierrors.ErrOktaLoginDenied
		}
		if !idTokenGroupsIntersectAllowed(groups, allowed) {
			return nil, apierrors.ErrOktaLoginDenied
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
	for _, b := range o.GroupRoleBindings {
		gn := strings.TrimSpace(b.GroupName)
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
