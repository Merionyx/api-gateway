// Package style provides minimal ANSI styling for CLI output (TTY + NO_COLOR aware).
package style

import (
	"io"
	"os"

	"golang.org/x/term"
)

// UseColorFor reports whether to emit ANSI sequences for w (typically stdout).
func UseColorFor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// S wraps s with seq when color is enabled.
func S(color bool, seq, s string) string {
	if !color {
		return s
	}
	return seq + s + "\033[0m"
}

// Common SGR fragments (caller passes through S).
const (
	Bold = "\033[1m"

	Dim = "\033[2m"

	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Black   = "\033[30m"

	BoldRed     = "\033[1;31m"
	BoldGreen   = "\033[1;32m"
	BoldYellow  = "\033[1;33m"
	BoldCyan    = "\033[1;36m"
	BoldGray    = "\033[1;90m"
	BoldBlue    = "\033[1;34m"
	BoldMagenta = "\033[1;35m"
	BoldBlack   = "\033[1;30m"
)
