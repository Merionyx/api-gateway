package auth

import (
	"fmt"
	"time"
)

type RequestedTokenTTLs struct {
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type TokenTTLPolicy struct {
	DefaultAccessTTL  time.Duration
	MaxAccessTTL      time.Duration
	DefaultRefreshTTL time.Duration
	MaxRefreshTTL     time.Duration
}

func resolveRequestedTokenTTLs(policy TokenTTLPolicy, requested RequestedTokenTTLs) (RequestedTokenTTLs, error) {
	accessTTL := policy.DefaultAccessTTL
	if requested.AccessTTL < 0 {
		return RequestedTokenTTLs{}, fmt.Errorf("requested_access_token_ttl_seconds must be > 0")
	}
	if requested.AccessTTL > 0 {
		accessTTL = requested.AccessTTL
	}
	if accessTTL > policy.MaxAccessTTL {
		accessTTL = policy.MaxAccessTTL
	}

	refreshTTL := policy.DefaultRefreshTTL
	if requested.RefreshTTL < 0 {
		return RequestedTokenTTLs{}, fmt.Errorf("requested_refresh_token_ttl_seconds must be > 0")
	}
	refreshExplicit := requested.RefreshTTL > 0
	if refreshExplicit {
		refreshTTL = requested.RefreshTTL
	}
	if refreshTTL > policy.MaxRefreshTTL {
		refreshTTL = policy.MaxRefreshTTL
	}
	if refreshTTL < accessTTL {
		if refreshExplicit {
			return RequestedTokenTTLs{}, fmt.Errorf("requested_refresh_token_ttl_seconds (%s) must be >= requested_access_token_ttl_seconds (%s)", refreshTTL, accessTTL)
		}
		refreshTTL = accessTTL
		if refreshTTL > policy.MaxRefreshTTL {
			return RequestedTokenTTLs{}, fmt.Errorf("requested access token ttl (%s) exceeds refresh policy maximum (%s)", accessTTL, policy.MaxRefreshTTL)
		}
	}

	return RequestedTokenTTLs{
		AccessTTL:  accessTTL,
		RefreshTTL: refreshTTL,
	}, nil
}
