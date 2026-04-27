package command

import (
	"strings"
	"testing"
	"time"
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
