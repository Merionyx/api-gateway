package git

import "testing"

func TestIsCommitHash(t *testing.T) {
	valid := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	if !isCommitHash(valid) {
		t.Fatal("expected valid 40-char hex")
	}
	if isCommitHash("short") {
		t.Fatal("short ref is not commit hash")
	}
	if isCommitHash(valid + "0") {
		t.Fatal("41 chars should not match")
	}
	if isCommitHash("gaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("non-hex should not match")
	}
}
