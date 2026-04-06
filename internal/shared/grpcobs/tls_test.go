package grpcobs

import "testing"

func TestServerTLS_Disabled(t *testing.T) {
	cfg, err := ServerTLS(ServerTLSConfig{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatalf("expected nil, got %v", cfg)
	}
}

func TestDialOptions_Insecure(t *testing.T) {
	opts, err := DialOptions(ClientTLSConfig{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) == 0 {
		t.Fatal("expected dial options")
	}
}
