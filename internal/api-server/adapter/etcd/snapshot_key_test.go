package etcd

import "testing"

func Test_bundleKeyFromSnapshotRelativeKey(t *testing.T) {
	for _, tt := range []struct {
		rel  string
		want string
		ok   bool
	}{
		{
			"api-gateway-schemas-https/remotes%2Forigin%2Fmaster/openapi/contracts/global.json",
			"api-gateway-schemas-https/remotes%2Forigin%2Fmaster/openapi",
			true,
		},
		{"nope", "", false},
		{"/contracts/x", "", false},
	} {
		got, ok := bundleKeyFromSnapshotRelativeKey(tt.rel)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("rel=%q got=%q ok=%v want=%q ok=%v", tt.rel, got, ok, tt.want, tt.ok)
		}
	}
}
