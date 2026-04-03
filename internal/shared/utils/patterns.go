package utils

import (
	"log"
	"regexp"
	"strings"
	"sync"
)

var patternCache sync.Map

// MatchesAnyEnvironment checks if the current environment matches any pattern in the list.
// Each pattern is a Go regexp (RE2). Patterns without leading ^ or trailing $ are anchored
// as a full-string match: "^(?:<pattern>)$". Examples: `dyn-test[123]`, `^dyn-test\d+$`.
func MatchesAnyEnvironment(currentEnv string, patterns []string) bool {
	for _, pattern := range patterns {
		if MatchesEnvironmentPattern(currentEnv, pattern) {
			return true
		}
	}
	return false
}

// MatchesEnvironmentPattern checks if the environment matches a pattern (see MatchesAnyEnvironment).
func MatchesEnvironmentPattern(env, pattern string) bool {
	if env == pattern {
		return true
	}

	src := regexpSourceForEnvironmentPattern(pattern)
	if cached, ok := patternCache.Load(src); ok {
		re, ok := cached.(*regexp.Regexp)
		if !ok || re == nil {
			log.Printf("[AUTH] Invalid cache entry for pattern %s: %#v", pattern, cached)
			return false
		}
		return re.MatchString(env)
	}

	compiled, err := regexp.Compile(src)
	if err != nil {
		log.Printf("[AUTH] Error compiling pattern %q: %v", pattern, err)
		return false
	}

	actual, _ := patternCache.LoadOrStore(src, compiled)
	re, ok := actual.(*regexp.Regexp)
	if !ok || re == nil {
		log.Printf("[AUTH] Invalid cache entry for pattern %q: %#v", pattern, actual)
		return false
	}
	return re.MatchString(env)
}

func regexpSourceForEnvironmentPattern(pattern string) string {
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		return pattern
	}
	var b strings.Builder
	b.Grow(len(pattern) + 6)
	b.WriteString("^(?:")
	b.WriteString(pattern)
	b.WriteString(")$")
	return b.String()
}
