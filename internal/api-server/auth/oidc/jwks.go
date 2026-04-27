package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
)

type jwksDocument struct {
	Keys []jwkRSA `json:"keys"`
}

type jwkRSA struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use,omitempty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func fetchRSAPublicKeys(ctx context.Context, hc *http.Client, jwksURI string) (map[string]*rsa.PublicKey, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("jwks: json: %w", err)
	}
	out := make(map[string]*rsa.PublicKey)
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		pub, err := rsaPublicFromJWKComponents(k.N, k.E)
		if err != nil {
			continue
		}
		if k.Kid != "" {
			out[k.Kid] = pub
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("jwks: no usable RSA keys")
	}
	return out, nil
}

func rsaPublicFromJWKComponents(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	eInt := 0
	for _, b := range eBytes {
		eInt = eInt<<8 | int(b)
	}
	if eInt < 2 {
		return nil, fmt.Errorf("jwks: invalid exponent")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}, nil
}
