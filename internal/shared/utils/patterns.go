package utils

import (
	"log"
	"regexp"
	"strings"
	"sync"
)

var (
	patternCache     sync.Map
	patternCompileMu sync.Mutex
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

		// Slow path: compile pattern at most once using double-checked locking
		patternCompileMu.Lock()
		defer patternCompileMu.Unlock()

		// Check cache again in case another goroutine stored it while we were waiting
		if cached, ok := patternCache.Load(pattern); ok {
			re, ok := cached.(*regexp.Regexp)
			if !ok || re == nil {
				log.Printf("[AUTH] Invalid cache entry for pattern %s: %#v", pattern, cached)
				return false
			}
			return re.MatchString(env)
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

		// Save to cache
		patternCache.Store(pattern, compiled)
		return compiled.MatchString(env)
	}
	return false
}
