package metricshttp

import "testing"

func TestListenAndServe_DisabledReturnsNil(t *testing.T) {
	if err := ListenAndServe(Config{Enabled: false}); err != nil {
		t.Fatal(err)
	}
}
