package utils

import (
	"testing"
)

func TestMatchesEnvironmentPattern_CorruptCacheEntry(t *testing.T) {
	patternCache.Clear()
	pattern := "svc"
	patternCache.Store("^(?:svc)$", "not-a-regexp")
	if MatchesEnvironmentPattern("other", pattern) {
		t.Fatal("corrupt cache entry must not match")
	}
}
