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

// MatchesAnyEnvironment checks if the current environment matches any pattern in the list
// Patterns can be exact matches or regex patterns (e.g., "dyn-*" becomes "^dyn-.*$")
func MatchesAnyEnvironment(currentEnv string, patterns []string) bool {
	for _, pattern := range patterns {
		if MatchesEnvironmentPattern(currentEnv, pattern) {
			return true
		}
	}
	return false
}

// MatchesEnvironmentPattern checks if the environment matches a pattern
// If pattern contains '*', it's treated as a wildcard pattern
// Otherwise, it's an exact match
func MatchesEnvironmentPattern(env, pattern string) bool {
	// Exact match
	if env == pattern {
		return true
	}
	// If pattern contains '*', treat it as a wildcard pattern
	if strings.Contains(pattern, "*") {

		// Fast path: check cache (lock-free read)
		if cached, ok := patternCache.Load(pattern); ok {
			re, ok := cached.(*regexp.Regexp)
			if !ok || re == nil {
				log.Printf("[AUTH] Invalid cache entry for pattern %s: %#v", pattern, cached)
				return false
			}
			return re.MatchString(env)
		}

		// Slow path: compile pattern and store atomically
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

		// Atomically load or store the compiled pattern
		actual, loaded := patternCache.LoadOrStore(pattern, compiled)
		re, ok := actual.(*regexp.Regexp)
		if !ok || re == nil {
			log.Printf("[AUTH] Invalid cache entry for pattern %s: %#v", pattern, actual)
			return false
		}

		// Use the regexp from the cache (whether newly stored or pre-existing)
		_ = loaded // loaded is not currently used, but kept for clarity and future use
		return re.MatchString(env)
	}
	return false
}
