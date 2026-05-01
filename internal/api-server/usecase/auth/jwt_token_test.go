package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

func TestJWTUseCase_GenerateToken(t *testing.T) {
	dir := t.TempDir()
	uc, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Hour)
	resp, err := uc.GenerateToken(context.Background(), &models.GenerateTokenRequest{
		AppID:        "app-1",
		Environments: []string{"dev"},
		ExpiresAt:    exp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" || resp.AppID != "app-1" {
		t.Fatalf("resp %+v", resp)
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(resp.Token, jwt.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	claims, _ := parsed.Claims.(jwt.MapClaims)
	if claims["iss"] != "edge-iss" || claims["aud"] != "edge-aud" {
		t.Fatalf("want edge iss/aud, got iss=%v aud=%v", claims["iss"], claims["aud"])
	}
}

func TestJWTUseCase_GenerateToken_NoActiveKey(t *testing.T) {
	uc := &JWTUseCase{
		edgeIssuer:      "e",
		edgeAudience:    "a",
		edgeSigningKeys: map[string]*KeyPair{},
		edgeActiveKeyID: "missing",
	}
	_, err := uc.GenerateToken(context.Background(), &models.GenerateTokenRequest{
		AppID:        "a",
		Environments: []string{"e"},
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
