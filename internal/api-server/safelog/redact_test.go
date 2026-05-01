package safelog

import (
	"strings"
	"testing"
)

func TestRedact_jwtAndBearer(t *testing.T) {
	t.Parallel()
	tok := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sigpart"
	in := "failed: " + tok + " hdr Authorization: Bearer abc.def.ghi"
	got := Redact(in)
	if got == in {
		t.Fatal("expected redaction")
	}
	if strings.Contains(got, tok) || strings.Contains(got, "abc.def.ghi") {
		t.Fatalf("leaked token material: %q", got)
	}
	if !strings.Contains(got, "[REDACTED_JWT]") || !strings.Contains(got, "Bearer [REDACTED]") {
		t.Fatalf("unexpected output: %q", got)
	}
}
