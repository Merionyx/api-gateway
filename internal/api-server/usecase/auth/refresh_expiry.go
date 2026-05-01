package auth

import (
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
)

func refreshSessionExpired(now, deadline time.Time) bool {
	if deadline.IsZero() {
		return true
	}
	return !now.Before(deadline)
}

func initialRefreshExpiry(now time.Time, ourTTL time.Duration, tr *oidc.TokenResponse) time.Time {
	deadline := now.Add(ourTTL)
	if providerDeadline, ok := providerRefreshExpiry(now, tr); ok && providerDeadline.Before(deadline) {
		return providerDeadline
	}
	return deadline
}

func rotatedRefreshExpiry(now time.Time, ourTTL time.Duration, previous time.Time, tr *oidc.TokenResponse) time.Time {
	deadline := now.Add(ourTTL)
	if providerDeadline, ok := providerRefreshExpiry(now, tr); ok {
		if providerDeadline.Before(deadline) {
			deadline = providerDeadline
		}
		return deadline
	}
	if !previous.IsZero() && previous.Before(deadline) {
		return previous
	}
	return deadline
}

func providerRefreshExpiry(now time.Time, tr *oidc.TokenResponse) (time.Time, bool) {
	if tr == nil {
		return time.Time{}, false
	}
	sec := minPositiveInt(tr.RefreshTokenExpiresIn, tr.RefreshExpiresIn)
	if sec <= 0 {
		return time.Time{}, false
	}
	return now.Add(time.Duration(sec) * time.Second), true
}

func minPositiveInt(values ...int) int {
	best := 0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if best == 0 || v < best {
			best = v
		}
	}
	return best
}
