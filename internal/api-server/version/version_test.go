package version

import "testing"

func TestAPISchemaVersion(t *testing.T) {
	t.Parallel()
	v := APISchemaVersion()
	if v == "" {
		t.Fatal("empty api schema version")
	}
}
