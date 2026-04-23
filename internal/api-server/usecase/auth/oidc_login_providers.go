package auth

import (
	"sort"
	"strings"
)

// OIDCProviderPublic is non-sensitive metadata for discovery (GET /api/v1/auth/oidc-providers).
type OIDCProviderPublic struct {
	ID     string
	Kind   string
	Issuer string
}

// ListPublicOIDCProviders returns configured providers sorted by id (stable for UIs and CLI).
func (u *OIDCLoginUseCase) ListPublicOIDCProviders() []OIDCProviderPublic {
	if len(u.byID) == 0 {
		return nil
	}
	out := make([]OIDCProviderPublic, 0, len(u.byID))
	for _, p := range u.byID {
		k := strings.TrimSpace(p.Kind)
		if k == "" {
			k = "generic"
		} else {
			k = strings.ToLower(k)
		}
		out = append(out, OIDCProviderPublic{
			ID:     strings.TrimSpace(p.ID),
			Kind:   k,
			Issuer: strings.TrimSpace(p.Issuer),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
