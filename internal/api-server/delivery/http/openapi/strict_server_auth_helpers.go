package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

func snapshotForAPIAccess(perms []any, mc jwt.MapClaims) ([]byte, error) {
	m := map[string]any{"omit_roles": true}
	if len(perms) > 0 {
		m["permissions"] = perms
	}
	if mc != nil {
		if s, _ := mc["idp_iss"].(string); strings.TrimSpace(s) != "" {
			m["idp_iss"] = strings.TrimSpace(s)
		}
		if s, _ := mc["idp_sub"].(string); strings.TrimSpace(s) != "" {
			m["idp_sub"] = strings.TrimSpace(s)
		}
	}
	return json.Marshal(m)
}

func resolveIssuedAPIAccessTTL(now time.Time, policyTTL time.Duration, callerClaims map[string]any, requestedExpiresAt *time.Time) (time.Duration, error) {
	callerExp, ok := numericUnixClaimToTime(callerClaims, "exp")
	if !ok {
		return 0, fmt.Errorf("caller token has no valid exp claim")
	}
	policyExp := now.Add(policyTTL)
	maxExp := policyExp
	if callerExp.Before(maxExp) {
		maxExp = callerExp
	}
	if !maxExp.After(now) {
		return 0, fmt.Errorf("caller token is too close to expiry")
	}

	targetExp := maxExp
	if requestedExpiresAt != nil {
		reqExp := requestedExpiresAt.UTC()
		if !reqExp.After(now) {
			return 0, fmt.Errorf("expires_at must be in the future")
		}
		if reqExp.After(maxExp) {
			return 0, fmt.Errorf("expires_at exceeds caller or policy limits")
		}
		targetExp = reqExp
	}
	ttl := targetExp.Sub(now)
	if ttl <= 0 {
		return 0, fmt.Errorf("computed token ttl is non-positive")
	}
	return ttl, nil
}

func normalizeRequestedPermissions(in *[]string) []string {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]string, 0, len(*in))
	seen := make(map[string]struct{}, len(*in))
	for i := range *in {
		s := strings.TrimSpace((*in)[i])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func permissionDescriptorsFromIDs(ids []string) []apiserver.PermissionDescriptor {
	if len(ids) == 0 {
		return []apiserver.PermissionDescriptor{}
	}
	unique := uniqueSortedStrings(ids)
	out := make([]apiserver.PermissionDescriptor, 0, len(unique))
	for _, permissionID := range unique {
		out = append(out, apiserver.PermissionDescriptor{
			Id:          permissionID,
			Description: permissions.Describe(permissionID),
		})
	}
	return out
}
