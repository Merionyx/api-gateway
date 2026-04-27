package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

type oauthIntentRepoStub struct {
	m map[string]kvvalue.LoginIntentValue
}

func (s *oauthIntentRepoStub) Get(_ context.Context, intentID string) (kvvalue.LoginIntentValue, error) {
	v, ok := s.m[intentID]
	if !ok {
		return kvvalue.LoginIntentValue{}, apierrors.ErrNotFound
	}
	return v, nil
}

func (s *oauthIntentRepoStub) Delete(_ context.Context, intentID string) error {
	delete(s.m, intentID)
	return nil
}

type oauthSessionRecord struct {
	v   kvvalue.SessionValue
	rev int64
}

type oauthSessionRepoStub struct {
	m map[string]oauthSessionRecord
}

func (s *oauthSessionRepoStub) Get(_ context.Context, sessionID string) (kvvalue.SessionValue, int64, error) {
	rec, ok := s.m[sessionID]
	if !ok {
		return kvvalue.SessionValue{}, 0, apierrors.ErrNotFound
	}
	return rec.v, rec.rev, nil
}

func (s *oauthSessionRepoStub) ReplaceCASWithRefreshIndex(_ context.Context, sessionID, oldVerifier, newVerifier string, v kvvalue.SessionValue, expectedModRevision int64) error {
	rec, ok := s.m[sessionID]
	if !ok {
		return apierrors.ErrNotFound
	}
	if rec.rev != expectedModRevision || strings.TrimSpace(rec.v.OurRefreshVerifier) != strings.TrimSpace(oldVerifier) {
		return etcd.ErrSessionCASConflict
	}
	rec.v = v
	rec.rev++
	s.m[sessionID] = rec
	return nil
}

func TestOAuthTokenUseCase_ExchangeAuthorizationCode_SuccessAndOneTimeCode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}

	codeVerifier := "verifier-value-123"
	sum := sha256.Sum256([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	code := "6ba7b810-9dad-41d4-a716-446655440001"

	intentRepo := &oauthIntentRepoStub{m: map[string]kvvalue.LoginIntentValue{
		code: {
			ProviderID:                      "p1",
			RedirectURI:                     "http://127.0.0.1:9999/v1/auth/callback",
			OAuthState:                      code,
			PKCEVerifier:                    "idp-pkce-verifier",
			OAuthClientID:                   "postman",
			OAuthClientRedirectURI:          "https://oauth.pstmn.io/v1/callback",
			OAuthClientState:                "client-state-1",
			OAuthClientCodeChallenge:        challenge,
			OAuthClientCodeChallengeMethod:  "S256",
			RequestedAccessTokenTTLSeconds:  3600,
			RequestedRefreshTokenTTLSeconds: 7200,
		},
	}}

	claims, err := json.Marshal(map[string]any{
		"sub": "user@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionRepo := &oauthSessionRepoStub{m: map[string]oauthSessionRecord{
		code: {
			v: kvvalue.SessionValue{
				ProviderID:         "p1",
				LoginIntentID:      code,
				OurRefreshVerifier: strings.Repeat("a", 64),
				ClaimsSnapshot:     claims,
				RefreshExpiresAt:   time.Now().UTC().Add(2 * time.Hour),
			},
			rev: 1,
		},
	}}

	uc := NewOAuthTokenUseCase(intentRepo, sessionRepo, jwtUC, nil, TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	})

	out, err := uc.Exchange(t.Context(), OAuthTokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  "https://oauth.pstmn.io/v1/callback",
		ClientID:     "postman",
		CodeVerifier: codeVerifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		t.Fatalf("empty access token: %+v", out)
	}
	if out.RefreshToken == nil || strings.TrimSpace(*out.RefreshToken) == "" {
		t.Fatalf("empty refresh token: %+v", out)
	}

	rec := sessionRepo.m[code]
	if rec.v.RotationGeneration != 1 {
		t.Fatalf("rotation_generation=%d", rec.v.RotationGeneration)
	}
	if rec.v.OurRefreshVerifier == strings.Repeat("a", 64) {
		t.Fatal("refresh verifier must be rotated")
	}

	_, err = uc.Exchange(t.Context(), OAuthTokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  "https://oauth.pstmn.io/v1/callback",
		ClientID:     "postman",
		CodeVerifier: codeVerifier,
	})
	if !errors.Is(err, apierrors.ErrSessionAuthFailed) {
		t.Fatalf("expected one-time code failure, got %v", err)
	}
}

func TestOAuthTokenUseCase_ExchangeAuthorizationCode_InvalidPKCE(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}

	code := "6ba7b810-9dad-41d4-a716-446655440002"
	intentRepo := &oauthIntentRepoStub{m: map[string]kvvalue.LoginIntentValue{
		code: {
			ProviderID:                     "p1",
			RedirectURI:                    "http://127.0.0.1:9999/v1/auth/callback",
			OAuthState:                     code,
			PKCEVerifier:                   "idp-pkce-verifier",
			OAuthClientID:                  "postman",
			OAuthClientRedirectURI:         "https://oauth.pstmn.io/v1/callback",
			OAuthClientCodeChallenge:       strings.Repeat("x", 43),
			OAuthClientCodeChallengeMethod: "S256",
		},
	}}
	claims, _ := json.Marshal(map[string]any{"sub": "user@example.com"})
	sessionRepo := &oauthSessionRepoStub{m: map[string]oauthSessionRecord{
		code: {
			v: kvvalue.SessionValue{
				ProviderID:         "p1",
				LoginIntentID:      code,
				OurRefreshVerifier: strings.Repeat("b", 64),
				ClaimsSnapshot:     claims,
				RefreshExpiresAt:   time.Now().UTC().Add(time.Hour),
			},
			rev: 1,
		},
	}}

	uc := NewOAuthTokenUseCase(intentRepo, sessionRepo, jwtUC, nil, TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	})

	_, err = uc.Exchange(t.Context(), OAuthTokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  "https://oauth.pstmn.io/v1/callback",
		ClientID:     "postman",
		CodeVerifier: "wrong-verifier",
	})
	if !errors.Is(err, apierrors.ErrSessionAuthFailed) {
		t.Fatalf("expected invalid grant, got %v", err)
	}
}
