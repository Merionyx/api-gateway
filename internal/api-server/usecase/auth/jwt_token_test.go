package auth

import (
	"context"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

func TestJWTUseCase_GenerateToken(t *testing.T) {
	dir := t.TempDir()
	uc, err := NewJWTUseCase(dir, "test-issuer")
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
}

func TestJWTUseCase_GenerateToken_NoActiveKey(t *testing.T) {
	uc := &JWTUseCase{
		issuer:      "i",
		signingKeys: map[string]*KeyPair{},
		activeKeyID: "missing",
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
