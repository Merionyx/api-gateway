package apiclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// TLSOptions configures HTTPS for requests to the API Server.
type TLSOptions struct {
	// Insecure skips TLS certificate verification (only use with --insecure).
	Insecure bool
	// CACertPath is a PEM file with one or more extra CA certificates (e.g. corporate root).
	// System certificate pool is used and these CAs are appended.
	CACertPath string
}

// NewHTTPClient returns an HTTP client with timeout and optional TLS settings.
// Default transport is used when no TLS options are set.
func NewHTTPClient(opts TLSOptions) (*http.Client, error) {
	timeout := 5 * time.Minute
	if opts.Insecure && opts.CACertPath != "" {
		return nil, fmt.Errorf("use either --insecure or --ca-cert, not both")
	}
	if !opts.Insecure && opts.CACertPath == "" {
		return &http.Client{Timeout: timeout}, nil
	}

	tr, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default transport is not *http.Transport")
	}
	t := tr.Clone()

	if opts.Insecure {
		t.TLSClientConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		}
		return &http.Client{Transport: t, Timeout: timeout}, nil
	}

	pemData, err := os.ReadFile(opts.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("read --ca-cert: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("no valid PEM certificates in --ca-cert file")
	}
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    pool,
	}
	t.TLSClientConfig = cfg
	return &http.Client{Transport: t, Timeout: timeout}, nil
}
