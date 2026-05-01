package command

import (
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
)

func TestParseOptionalTTLFlag_AcceptsDuration(t *testing.T) {
	t.Parallel()

	got, err := parseOptionalTTLFlag("--access-ttl", "168h")
	if err != nil {
		t.Fatalf("parseOptionalTTLFlag: %v", err)
	}
	if got != 168*time.Hour {
		t.Fatalf("ttl = %s", got)
	}
}

func TestParseOptionalTTLFlag_AcceptsIntegerSeconds(t *testing.T) {
	t.Parallel()

	got, err := parseOptionalTTLFlag("--access-ttl", "86400")
	if err != nil {
		t.Fatalf("parseOptionalTTLFlag: %v", err)
	}
	if got != 24*time.Hour {
		t.Fatalf("ttl = %s", got)
	}
}

func TestParseOptionalTTLFlag_RejectsSubSecond(t *testing.T) {
	t.Parallel()

	_, err := parseOptionalTTLFlag("--access-ttl", "1500ms")
	if err == nil || !strings.Contains(err.Error(), "whole number of seconds") {
		t.Fatalf("expected whole-seconds error, got %v", err)
	}
}

func TestParseOptionalTTLFlag_RejectsInvalid(t *testing.T) {
	t.Parallel()

	_, err := parseOptionalTTLFlag("--access-ttl", "abc")
	if err == nil || !strings.Contains(err.Error(), "time: invalid duration") {
		t.Fatalf("expected invalid duration error, got %v", err)
	}
}

func TestWithDefaultRequestedTTLs(t *testing.T) {
	t.Parallel()

	got := withDefaultRequestedTTLs(httpapi.RequestedTokenTTLs{})
	if got.AccessTTL != 7*24*time.Hour || got.RefreshTTL != 30*24*time.Hour {
		t.Fatalf("ttls = %+v", got)
	}
}

func TestTTLString(t *testing.T) {
	t.Parallel()

	if got := ttlString(7 * 24 * time.Hour); got != "168h" {
		t.Fatalf("ttlString = %q", got)
	}
	if got := ttlString(15 * time.Minute); got != "15m" {
		t.Fatalf("ttlString = %q", got)
	}
	if got := ttlString(45 * time.Second); got != "45s" {
		t.Fatalf("ttlString = %q", got)
	}
}
