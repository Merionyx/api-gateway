package pkce

import (
	"strings"
	"testing"
)

// RFC 7636 appendix B verifier; challenge matches Go crypto/sha256 + base64.RawURLEncoding (some published copies omit the final "M").
func TestChallengeS256_RFC7636AppendixB(t *testing.T) {
	t.Parallel()
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := ChallengeS256(verifier); got != want {
		t.Fatalf("challenge: got %q want %q", got, want)
	}
}

func TestNewVerifier_LengthAndCharset(t *testing.T) {
	t.Parallel()
	v, err := NewVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) < 43 {
		t.Fatalf("verifier too short: %d %q", len(v), v)
	}
	for _, ch := range v {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' {
			continue
		}
		t.Fatalf("unexpected rune %q in %q", ch, v)
	}
	if strings.Contains(v, "=") {
		t.Fatalf("padding must be omitted: %q", v)
	}
}
