package validate

import "testing"

func TestParseRoot(t *testing.T) {
	t.Parallel()
	m, err := ParseRoot([]byte(`a: 1`))
	if err != nil || m["a"] != 1 {
		t.Fatalf("got %v %v", m, err)
	}
	if _, err := ParseRoot([]byte(`null`)); err == nil {
		t.Fatal("expected empty document error")
	}
}
