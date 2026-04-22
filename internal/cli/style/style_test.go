package style

import (
	"bytes"
	"testing"
)

func TestS(t *testing.T) {
	t.Parallel()
	if s := S(false, Red, "x"); s != "x" {
		t.Fatalf("%q", s)
	}
	if s := S(true, Cyan, "k"); s == "k" || !bytes.Contains([]byte(s), []byte("\033[0m")) {
		t.Fatalf("%q", s)
	}
}

func TestUseColorFor_NonFileWriter(t *testing.T) {
	t.Parallel()
	if UseColorFor(&bytes.Buffer{}) {
		t.Fatal("not a file")
	}
}
