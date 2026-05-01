package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func googleExtraRoles(g *config.GoogleOIDCProviderConfig, mc jwt.MapClaims) ([]string, error) {
	if g == nil {
		g = &config.GoogleOIDCProviderConfig{}
	}
	allowedHD := stringSetLowerTrim(g.AllowedHostedDomains)
	allowedEmailDom := stringSetLowerTrim(g.AllowedEmailDomains)
	needHD := len(allowedHD) > 0
	needEmail := len(allowedEmailDom) > 0
	needBindHD := len(g.HostedDomainRoleBindings) > 0
	needBindEmail := len(g.EmailDomainRoleBindings) > 0
	if !needHD && !needEmail && !needBindHD && !needBindEmail {
		return nil, nil
	}

	email := strings.TrimSpace(googleStringClaim(mc, "email"))
	hd := strings.TrimSpace(googleStringClaim(mc, "hd"))

	if needHD {
		if hd == "" {
			return nil, apierrors.ErrGoogleLoginDenied
		}
		if _, ok := allowedHD[strings.ToLower(hd)]; !ok {
			return nil, apierrors.ErrGoogleLoginDenied
		}
	} else if needEmail {
		d := emailDomainFromAddress(email)
		if d == "" {
			return nil, apierrors.ErrGoogleLoginDenied
		}
		if _, ok := allowedEmailDom[d]; !ok {
			return nil, apierrors.ErrGoogleLoginDenied
		}
	}

	var extras []string
	for _, b := range g.HostedDomainRoleBindings {
		bhd := strings.TrimSpace(b.HD)
		if bhd == "" {
			continue
		}
		if strings.EqualFold(hd, bhd) {
			for _, r := range b.Roles {
				if s := strings.TrimSpace(r); s != "" {
					extras = append(extras, s)
				}
			}
		}
	}
	dom := emailDomainFromAddress(email)
	for _, b := range g.EmailDomainRoleBindings {
		d := strings.TrimSpace(strings.ToLower(b.Domain))
		if d == "" {
			continue
		}
		if dom == d {
			for _, r := range b.Roles {
				if s := strings.TrimSpace(r); s != "" {
					extras = append(extras, s)
				}
			}
		}
	}
	return extras, nil
}
