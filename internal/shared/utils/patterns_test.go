package utils

import (
	"testing"
)

func TestMatchesEnvironmentPatternLiteral(t *testing.T) {
	if !MatchesEnvironmentPattern("prod", "prod") {
		t.Error("exact literal should match")
	}
	if MatchesEnvironmentPattern("prod", "staging") {
		t.Error("different literals should not match")
	}
}

func TestMatchesEnvironmentPatternAnchoredImplicit(t *testing.T) {
	// Without ^/$ the pattern is wrapped as full-string match.
	if !MatchesEnvironmentPattern("dyn-test1", `dyn-test[123]`) {
		t.Error("char class should match single digit variant")
	}
	if MatchesEnvironmentPattern("dyn-test12", `dyn-test[123]`) {
		t.Error("two trailing digits should not match single-char class")
	}
	if MatchesEnvironmentPattern("xdyn-test1", `dyn-test[123]`) {
		t.Error("prefix should not match without leading .*")
	}
}

func TestMatchesEnvironmentPatternExplicitAnchors(t *testing.T) {
	if !MatchesEnvironmentPattern("dyn-test99", `^dyn-test\d+$`) {
		t.Error("explicit anchors with digits")
	}
	if MatchesEnvironmentPattern("dyn-test", `^dyn-test\d+$`) {
		t.Error("missing digit suffix should not match")
	}
}

func TestMatchesEnvironmentPatternInvalidRegexp(t *testing.T) {
	if MatchesEnvironmentPattern("anything", "(") {
		t.Error("invalid pattern should yield false, not match")
	}
}

func TestMatchesAnyEnvironment(t *testing.T) {
	if !MatchesAnyEnvironment("dev", []string{"staging", "dev", "prod"}) {
		t.Error("should match second pattern")
	}
	if MatchesAnyEnvironment("qa", []string{"staging", "dev"}) {
		t.Error("should match none")
	}
	if MatchesAnyEnvironment("x", nil) {
		t.Error("empty pattern list should not match")
	}
	if !MatchesAnyEnvironment("prod", []string{"prod"}) {
		t.Error("single pattern list")
	}
}
