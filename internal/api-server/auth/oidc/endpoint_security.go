package oidc

import (
	"fmt"
	"net/url"
	"strings"
)

func validateEndpointURL(raw, endpointName string, allowInsecure bool) error {
	v := strings.TrimSpace(raw)
	u, err := url.Parse(v)
	if err != nil || strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("%w: %s must be an absolute URL", ErrInsecureEndpoint, endpointName)
	}
	if allowInsecure {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(u.Scheme), "https") {
		return fmt.Errorf("%w: %s must use https", ErrInsecureEndpoint, endpointName)
	}
	return nil
}
