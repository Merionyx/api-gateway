package config

import "testing"

func TestAuthConfig_BootstrapAPIKeyAllowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		cfg    AuthConfig
		allow  bool
	}{
		{"off", AuthConfig{AllowInsecureBootstrap: false, Environment: "development"}, false},
		{"prod flag on still blocked", AuthConfig{AllowInsecureBootstrap: true, Environment: "production"}, false},
		{"staging blocked", AuthConfig{AllowInsecureBootstrap: true, Environment: "staging"}, false},
		{"dev ok", AuthConfig{AllowInsecureBootstrap: true, Environment: "development"}, true},
		{"local ok", AuthConfig{AllowInsecureBootstrap: true, Environment: "local"}, true},
		{"dev case", AuthConfig{AllowInsecureBootstrap: true, Environment: "Development"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.cfg.BootstrapAPIKeyAllowed(); got != tc.allow {
				t.Fatalf("BootstrapAPIKeyAllowed()=%v want %v", got, tc.allow)
			}
		})
	}
}
