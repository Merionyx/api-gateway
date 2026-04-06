package server

import (
	"testing"

	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

func TestDenyResponse_StatusCodes(t *testing.T) {
	for _, tc := range []struct {
		code     int
		wantEnum typev3.StatusCode
	}{
		{401, typev3.StatusCode_Unauthorized},
		{403, typev3.StatusCode_Forbidden},
		{404, typev3.StatusCode_Unauthorized},
	} {
		resp := denyResponse("x", tc.code)
		denied := resp.GetDeniedResponse()
		if denied == nil {
			t.Fatalf("code %d: no denied", tc.code)
		}
		if denied.GetStatus().GetCode() != tc.wantEnum {
			t.Fatalf("code %d: got %v want %v", tc.code, denied.GetStatus().GetCode(), tc.wantEnum)
		}
	}
}

func TestAllowResponse_Headers(t *testing.T) {
	resp := allowResponse("app1", "c1")
	ok := resp.GetOkResponse()
	if ok == nil {
		t.Fatal("nil ok")
	}
	if len(ok.GetHeaders()) < 2 {
		t.Fatalf("headers: %d", len(ok.GetHeaders()))
	}
}
