// Package safelog provides string redaction for logs (roadmap ш. 1, 25): avoid raw JWTs and Bearer material in slog output.
package safelog

import "regexp"

var (
	reCompactJWT = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	reBearer     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~-]+\b`)
)

// Redact replaces JWT-shaped substrings and `Bearer <token>` fragments. Best-effort; does not parse JWT.
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = reCompactJWT.ReplaceAllString(s, "[REDACTED_JWT]")
	s = reBearer.ReplaceAllString(s, "Bearer [REDACTED]")
	return s
}
