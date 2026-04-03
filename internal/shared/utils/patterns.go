package utils

import (
	"log"
	"regexp"
	"strings"
	"sync"
)

var (
	patternCache sync.Map
)

// matchesAnyEnvironment checks if the current environment matches any pattern in the list
// Patterns can be exact matches or regex patterns (e.g., "dyn-*" becomes "^dyn-.*$")
func MatchesAnyEnvironment(currentEnv string, patterns []string) bool {
	for _, pattern := range patterns {
		if MatchesEnvironmentPattern(currentEnv, pattern) {
			return true
		}
	}
	return false
}

// matchesEnvironmentPattern checks if the environment matches a pattern
// If pattern contains '*', it's treated as a wildcard pattern
// Otherwise, it's an exact match
func MatchesEnvironmentPattern(env, pattern string) bool {
	// Exact match
	if env == pattern {
		return true
	}
	// If pattern contains '*', treat it as a wildcard pattern
	if strings.Contains(pattern, "*") {
		// Check cache (lock-free read!)
		if cached, ok := patternCache.Load(pattern); ok {
			return cached.(*regexp.Regexp).MatchString(env)
		}
		// Compile new pattern
		regexPattern := regexp.QuoteMeta(pattern)
		regexPattern = strings.ReplaceAll(regexPattern, "\\*", ".*")
		var builder strings.Builder
		builder.Grow(len(regexPattern) + 2)
		builder.WriteString("^")
		builder.WriteString(regexPattern)
		builder.WriteString("$")
		compiled, err := regexp.Compile(builder.String())
		if err != nil {
			log.Printf("[AUTH] Error compiling pattern %s: %v", pattern, err)
			return false
		}
		// Save to cache (atomic operation!)
		// LoadOrStore returns the existing value if someone else has already stored it
		actual, _ := patternCache.LoadOrStore(pattern, compiled)
		return actual.(*regexp.Regexp).MatchString(env)
	}
	return false
}
