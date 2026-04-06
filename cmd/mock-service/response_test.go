package main

import (
	"encoding/json"
	"testing"
)

func TestResponse_JSONRoundTrip(t *testing.T) {
	r := Response{
		Service: "svc", Environment: "dev", Path: "/p", Method: "GET",
		Headers: map[string]string{"a": "b"},
		Query:   map[string]string{"q": "1"},
		Host:    "h",
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out Response
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Service != r.Service || out.Path != r.Path {
		t.Fatalf("%+v", out)
	}
}
