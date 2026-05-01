package auth

import (
	"strings"
	"testing"
	"time"
)

func TestResolveRequestedTokenTTLs_Defaults(t *testing.T) {
	t.Parallel()
	got, err := resolveRequestedTokenTTLs(TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, RequestedTokenTTLs{})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessTTL != 5*time.Minute || got.RefreshTTL != 7*24*time.Hour {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveRequestedTokenTTLs_ClampsToMax(t *testing.T) {
	t.Parallel()
	got, err := resolveRequestedTokenTTLs(TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, RequestedTokenTTLs{
		AccessTTL:  14 * 24 * time.Hour,
		RefreshTTL: 90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessTTL != 7*24*time.Hour || got.RefreshTTL != 30*24*time.Hour {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveRequestedTokenTTLs_ImplicitRefreshFollowsAccess(t *testing.T) {
	t.Parallel()
	got, err := resolveRequestedTokenTTLs(TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, RequestedTokenTTLs{
		AccessTTL: 10 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessTTL != 7*24*time.Hour || got.RefreshTTL != 7*24*time.Hour {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveRequestedTokenTTLs_RejectsExplicitRefreshShorterThanAccess(t *testing.T) {
	t.Parallel()
	_, err := resolveRequestedTokenTTLs(TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, RequestedTokenTTLs{
		AccessTTL:  2 * time.Hour,
		RefreshTTL: time.Hour,
	})
	if err == nil || !strings.Contains(err.Error(), "refresh_ttl") {
		t.Fatalf("got %v", err)
	}
}
