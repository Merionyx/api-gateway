package auth

import (
	"crypto"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/config"
)

const (
	AlgorithmEdDSA = "EdDSA"
	AlgorithmRS256 = "RS256"
)

// JWTUseCase issues Edge vs API JWT profiles with separate key directories .
type JWTUseCase struct {
	apiKeysDir   string
	edgeKeysDir  string
	apiIssuer    string
	apiAudience  string
	edgeIssuer   string
	edgeAudience string

	apiSigningKeys map[string]*KeyPair
	apiActiveKeyID string

	edgeSigningKeys map[string]*KeyPair
	edgeActiveKeyID string
}

// KeyPair holds a signing identity loaded from disk or generated at startup.
type KeyPair struct {
	Kid        string
	Algorithm  string
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
	CreatedAt  time.Time
}

// NewJWTUseCase loads API and Edge signing keys from jwt.keys_dir and jwt.edge_keys_dir (default: keys_dir/edge).
func NewJWTUseCase(cfg *config.JWTConfig) (*JWTUseCase, error) {
	if cfg == nil {
		return nil, errors.New("jwt: nil config")
	}
	apiDir := strings.TrimSpace(cfg.KeysDir)
	if apiDir == "" {
		return nil, errors.New("jwt: keys_dir is required")
	}
	edgeDir := strings.TrimSpace(cfg.EdgeKeysDir)
	if edgeDir == "" {
		edgeDir = filepath.Join(apiDir, "edge")
	}

	apiIss := strings.TrimSpace(cfg.Issuer)
	if apiIss == "" {
		return nil, errors.New("jwt: issuer is required")
	}
	apiAud := strings.TrimSpace(cfg.APIAudience)
	if apiAud == "" {
		apiAud = apiIss + "#api"
	}
	edgeIss := strings.TrimSpace(cfg.EdgeIssuer)
	if edgeIss == "" {
		edgeIss = "api-gateway-edge"
	}
	edgeAud := strings.TrimSpace(cfg.EdgeAudience)
	if edgeAud == "" {
		edgeAud = "api-gateway-edge-http"
	}

	uc := &JWTUseCase{
		apiKeysDir:      apiDir,
		edgeKeysDir:     edgeDir,
		apiIssuer:       apiIss,
		apiAudience:     apiAud,
		edgeIssuer:      edgeIss,
		edgeAudience:    edgeAud,
		apiSigningKeys:  make(map[string]*KeyPair),
		edgeSigningKeys: make(map[string]*KeyPair),
	}

	apiKeys, apiNewest, _, err := loadKeyDirectory(uc.apiKeysDir)
	if err != nil {
		return nil, fmt.Errorf("jwt api keys: %w", err)
	}
	if len(apiKeys) == 0 {
		kp, gerr := generateDefaultEd25519InDir(uc.apiKeysDir, "api-server-key")
		if gerr != nil {
			return nil, fmt.Errorf("jwt api default key: %w", gerr)
		}
		apiKeys[kp.Kid] = kp
		apiNewest = kp.Kid
	}
	apiActive, err := resolveSigningKeyID("api", apiKeys, cfg.APISigningKid, apiNewest)
	if err != nil {
		return nil, fmt.Errorf("jwt api keys: %w", err)
	}
	uc.apiSigningKeys = apiKeys
	uc.apiActiveKeyID = apiActive

	edgeKeys, edgeNewest, _, err := loadKeyDirectory(uc.edgeKeysDir)
	if err != nil {
		return nil, fmt.Errorf("jwt edge keys: %w", err)
	}
	if len(edgeKeys) == 0 {
		kp, gerr := generateDefaultEd25519InDir(uc.edgeKeysDir, "edge-server-key")
		if gerr != nil {
			return nil, fmt.Errorf("jwt edge default key: %w", gerr)
		}
		edgeKeys[kp.Kid] = kp
		edgeNewest = kp.Kid
	}
	edgeActive, err := resolveSigningKeyID("edge", edgeKeys, cfg.EdgeSigningKid, edgeNewest)
	if err != nil {
		return nil, fmt.Errorf("jwt edge keys: %w", err)
	}
	uc.edgeSigningKeys = edgeKeys
	uc.edgeActiveKeyID = edgeActive

	return uc, nil
}
