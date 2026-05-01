package command

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/credentials"
)

const (
	defaultRequestedAccessTTL  = 7 * 24 * time.Hour
	defaultRequestedRefreshTTL = 30 * 24 * time.Hour
)

func parseOptionalTTLFlag(flagName, raw string) (time.Duration, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, nil
	}
	d, err := parseTTLValue(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", flagName, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be > 0", flagName)
	}
	if d%time.Second != 0 {
		return 0, fmt.Errorf("%s must be a whole number of seconds", flagName)
	}
	return d, nil
}

func parseTTLValue(raw string) (time.Duration, error) {
	d, parseErr := time.ParseDuration(raw)
	if parseErr == nil {
		return d, nil
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, parseErr
	}
	if secs > math.MaxInt64/int64(time.Second) {
		return 0, fmt.Errorf("duration seconds %d overflows Go time.Duration", secs)
	}
	return time.Duration(secs) * time.Second, nil
}

func withDefaultRequestedTTLs(ttls httpapi.RequestedTokenTTLs) httpapi.RequestedTokenTTLs {
	if ttls.AccessTTL <= 0 {
		ttls.AccessTTL = defaultRequestedAccessTTL
	}
	if ttls.RefreshTTL <= 0 {
		ttls.RefreshTTL = defaultRequestedRefreshTTL
	}
	return ttls
}

func ttlString(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	default:
		return d.String()
	}
}

func resolvedTTLString(raw string, resolved time.Duration) string {
	if s := strings.TrimSpace(raw); s != "" {
		return s
	}
	return ttlString(resolved)
}

func requestedTTLsFromFlags(accessFlag, refreshFlag string) (httpapi.RequestedTokenTTLs, error) {
	accessTTL, err := parseOptionalTTLFlag("--access-ttl", accessFlag)
	if err != nil {
		return httpapi.RequestedTokenTTLs{}, err
	}
	refreshTTL, err := parseOptionalTTLFlag("--refresh-ttl", refreshFlag)
	if err != nil {
		return httpapi.RequestedTokenTTLs{}, err
	}
	return httpapi.RequestedTokenTTLs{AccessTTL: accessTTL, RefreshTTL: refreshTTL}, nil
}

func requestedTTLsFromCredentials(saved credentials.Entry) (httpapi.RequestedTokenTTLs, error) {
	accessTTL, err := parseOptionalTTLFlag("saved requested_access_token_ttl", saved.RequestedAccessTokenTTL)
	if err != nil {
		return httpapi.RequestedTokenTTLs{}, err
	}
	refreshTTL, err := parseOptionalTTLFlag("saved requested_refresh_token_ttl", saved.RequestedRefreshTokenTTL)
	if err != nil {
		return httpapi.RequestedTokenTTLs{}, err
	}
	return httpapi.RequestedTokenTTLs{AccessTTL: accessTTL, RefreshTTL: refreshTTL}, nil
}

func optionalSeconds(d time.Duration) *int {
	if d <= 0 {
		return nil
	}
	v := int(d / time.Second)
	return &v
}
