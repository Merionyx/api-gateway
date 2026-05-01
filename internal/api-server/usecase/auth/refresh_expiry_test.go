package auth

import (
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
)

func TestInitialRefreshExpiry_ClampsToProvider(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	got := initialRefreshExpiry(now, 30*24*time.Hour, &oidc.TokenResponse{RefreshTokenExpiresIn: 7 * 24 * 3600})
	want := now.Add(7 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRotatedRefreshExpiry_KeepsPreviousWhenProviderSilent(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	prev := now.Add(72 * time.Hour)
	got := rotatedRefreshExpiry(now, 30*24*time.Hour, prev, &oidc.TokenResponse{})
	if !got.Equal(prev) {
		t.Fatalf("got %s want %s", got, prev)
	}
}

func TestRotatedRefreshExpiry_UsesNewProviderDeadlineWhenKnown(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	prev := now.Add(24 * time.Hour)
	got := rotatedRefreshExpiry(now, 30*24*time.Hour, prev, &oidc.TokenResponse{RefreshExpiresIn: 10 * 24 * 3600})
	want := now.Add(10 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestRefreshSessionExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	if !refreshSessionExpired(now, time.Time{}) {
		t.Fatal("zero deadline must be treated as expired")
	}
	if refreshSessionExpired(now, now.Add(time.Second)) {
		t.Fatal("future deadline must be active")
	}
	if !refreshSessionExpired(now, now) {
		t.Fatal("exact deadline must be expired")
	}
}
