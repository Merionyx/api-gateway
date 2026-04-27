package command

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestProviderDisplayName(t *testing.T) {
	t.Parallel()

	if got := providerDisplayName(apiserverclient.OidcProviderDescriptor{Name: "GitHub Enterprise", Id: "github-enterprise"}); got != "GitHub Enterprise" {
		t.Fatalf("got %q", got)
	}
	if got := providerDisplayName(apiserverclient.OidcProviderDescriptor{Id: "github"}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestProviderSelectLabel(t *testing.T) {
	t.Parallel()

	got := providerSelectLabel(apiserverclient.OidcProviderDescriptor{
		Id:   "github-enterprise",
		Name: "GitHub Enterprise",
		Kind: "github",
	})
	if got != "GitHub Enterprise  \x1b[2m(github-enterprise)\x1b[0m" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveAuthLoginProviderID_UsesProviderNameForSingleProvider(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/v1/auth/oidc-providers" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("not found")),
					Request:    r,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`[{"id":"github-enterprise","name":"GitHub Enterprise","kind":"github","issuer":"https://github.example.com"}]`)),
				Request:    r,
			}, nil
		}),
	}
	got, err := resolveAuthLoginProviderID(context.Background(), "http://api.example", httpClient, "", &out)
	if err != nil {
		t.Fatal(err)
	}
	if got != "github-enterprise" {
		t.Fatalf("got %q", got)
	}
	if out.String() != "Using the only configured provider \"GitHub Enterprise\" [github-enterprise] (github).\n" {
		t.Fatalf("output %q", out.String())
	}
}
