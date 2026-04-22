package telemetry

import "testing"

func TestSkipProbePath(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"/health":         true,
		"/ready":          true,
		"/v1/health":      true,
		"/api/v1/ready":   true,
		"/":               false,
		"/v1/healthcheck": false, // must not match: path does not end with /health
		"/v1/health/":     false,
		"/readiness":      false, // not suffix /ready
		"/v1/contract":    false,
	}
	for p, want := range cases {
		if got := SkipProbePath(p); got != want {
			t.Fatalf("SkipProbePath(%q) = %v, want %v", p, got, want)
		}
	}
}
