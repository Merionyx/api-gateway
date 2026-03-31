package usecase

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"time"

	"merionyx/api-gateway/internal/api-server/domain/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AlgorithmEdDSA = "EdDSA"
	AlgorithmRS256 = "RS256"
)

type JWTUseCase struct {
	keysDir      string
	issuer       string
	signingKeys  map[string]*KeyPair // kid -> KeyPair
	activeKeyID  string
	activeKeyAlg string
}

type KeyPair struct {
	Kid        string
	Algorithm  string
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
	CreatedAt  time.Time
}

func NewJWTUseCase(keysDir, issuer string) (*JWTUseCase, error) {
	uc := &JWTUseCase{
		keysDir:     keysDir,
		issuer:      issuer,
		signingKeys: make(map[string]*KeyPair),
	}

	// Load all keys from the directory
	if err := uc.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	// If there are no keys - create a default EdDSA key
	if len(uc.signingKeys) == 0 {
		if err := uc.generateDefaultKey(); err != nil {
			return nil, fmt.Errorf("failed to generate default key: %w", err)
		}
	}

	return uc, nil
}

// GenerateToken generates a JWT token
func (uc *JWTUseCase) GenerateToken(req *models.GenerateTokenRequest) (*models.GenerateTokenResponse, error) {
	now := time.Now()
	tokenID := uuid.New().String()

	// Create claims
	claims := jwt.MapClaims{
		"iss":    uc.issuer,
		"sub":    req.AppID,
		"app_id": req.AppID,
		"iat":    now.Unix(),
		"exp":    req.ExpiresAt.Unix(),
		"jti":    tokenID,
	}

	if req.Environment != "" {
		claims["environment"] = req.Environment
	}

	// Get the active key
	keyPair := uc.signingKeys[uc.activeKeyID]
	if keyPair == nil {
		return nil, fmt.Errorf("no active signing key found")
	}

	// Create a token with the correct algorithm
	var token *jwt.Token
	switch keyPair.Algorithm {
	case AlgorithmEdDSA:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	case AlgorithmRS256:
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", keyPair.Algorithm)
	}

	// Add the kid to the header
	token.Header["kid"] = keyPair.Kid

	// Sign the token
	tokenString, err := token.SignedString(keyPair.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	response := &models.GenerateTokenResponse{
		ID:          tokenID,
		Token:       tokenString,
		AppID:       req.AppID,
		Environment: req.Environment,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   now,
	}

	if req.Environment != "" {
		response.Environment = req.Environment
	}

	return response, nil
}

// GetJWKS returns a JSON Web Key Set with all public keys
func (uc *JWTUseCase) GetJWKS() (*models.JWKS, error) {
	jwks := &models.JWKS{
		Keys: make([]models.JWK, 0),
	}

	// Sort keys by kid for stable order
	kids := make([]string, 0, len(uc.signingKeys))
	for kid := range uc.signingKeys {
		kids = append(kids, kid)
	}
	sort.Strings(kids)

	for _, kid := range kids {
		keyPair := uc.signingKeys[kid]

		jwk, err := uc.publicKeyToJWK(keyPair)
		if err != nil {
			return nil, fmt.Errorf("failed to convert key %s to JWK: %w", kid, err)
		}

		jwks.Keys = append(jwks.Keys, *jwk)
	}

	return jwks, nil
}

// GetSigningKeys returns a list of signing keys
func (uc *JWTUseCase) GetSigningKeys() []models.SigningKey {
	keys := make([]models.SigningKey, 0, len(uc.signingKeys))

	for kid, keyPair := range uc.signingKeys {
		keys = append(keys, models.SigningKey{
			Kid:       kid,
			Algorithm: keyPair.Algorithm,
			Active:    kid == uc.activeKeyID,
			CreatedAt: keyPair.CreatedAt,
		})
	}

	// Sort by creation date (newest first)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})

	return keys
}

// loadKeys loads all keys from the directory
func (uc *JWTUseCase) loadKeys() error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(uc.keysDir, 0700); err != nil {
		return err
	}

	// Read all files in the directory
	files, err := os.ReadDir(uc.keysDir)
	if err != nil {
		return err
	}

	var newestKey *KeyPair
	var newestTime time.Time

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Search for private keys
		if filepath.Ext(file.Name()) == ".key" {
			keyPath := filepath.Join(uc.keysDir, file.Name())

			keyPair, err := uc.loadKeyPair(keyPath)
			if err != nil {
				fmt.Printf("Warning: failed to load key %s: %v\n", file.Name(), err)
				continue
			}

			uc.signingKeys[keyPair.Kid] = keyPair

			// Track the newest key
			if keyPair.CreatedAt.After(newestTime) {
				newestTime = keyPair.CreatedAt
				newestKey = keyPair
			}
		}
	}

	// Set the newest key as active
	if newestKey != nil {
		uc.activeKeyID = newestKey.Kid
		uc.activeKeyAlg = newestKey.Algorithm
		fmt.Printf("Active signing key: %s (%s)\n", uc.activeKeyID, uc.activeKeyAlg)
	}

	return nil
}

// loadKeyPair loads a key pair from a file
func (uc *JWTUseCase) loadKeyPair(keyPath string) (*KeyPair, error) {
	// Read private key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	var privateKey crypto.PrivateKey
	var publicKey crypto.PublicKey
	var algorithm string

	switch block.Type {
	case "PRIVATE KEY":
		// PKCS#8 format - can be Ed25519 or RSA
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		switch k := key.(type) {
		case ed25519.PrivateKey:
			privateKey = k
			publicKey = k.Public()
			algorithm = AlgorithmEdDSA
		case *rsa.PrivateKey:
			privateKey = k
			publicKey = &k.PublicKey
			algorithm = AlgorithmRS256
		default:
			return nil, fmt.Errorf("unsupported key type: %T", k)
		}

	case "RSA PRIVATE KEY":
		// PKCS#1 format - can be RSA
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		privateKey = key
		publicKey = &key.PublicKey
		algorithm = AlgorithmRS256

	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}

	// Extract kid from the file name (without extension)
	kid := filepath.Base(keyPath)
	kid = kid[:len(kid)-len(filepath.Ext(kid))]

	// Get the creation time of the file
	fileInfo, err := os.Stat(keyPath)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		Kid:        kid,
		Algorithm:  algorithm,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  fileInfo.ModTime(),
	}, nil
}

// generateDefaultKey generates a default EdDSA key
func (uc *JWTUseCase) generateDefaultKey() error {
	kid := fmt.Sprintf("api-server-key-%s", time.Now().Format("2006-01-02"))
	keyPath := filepath.Join(uc.keysDir, kid+".key")

	// Generate Ed25519 key
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	// Encode in PKCS#8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	// Save in PEM format
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		return err
	}

	// Add to the list of keys
	uc.signingKeys[kid] = &KeyPair{
		Kid:        kid,
		Algorithm:  AlgorithmEdDSA,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  time.Now(),
	}

	uc.activeKeyID = kid
	uc.activeKeyAlg = AlgorithmEdDSA

	fmt.Printf("Generated default EdDSA key: %s\n", kid)

	return nil
}

// publicKeyToJWK converts a public key to JWK format
func (uc *JWTUseCase) publicKeyToJWK(keyPair *KeyPair) (*models.JWK, error) {
	jwk := &models.JWK{
		Kid: keyPair.Kid,
		Use: "sig",
	}

	switch keyPair.Algorithm {
	case AlgorithmEdDSA:
		// Ed25519
		edKey, ok := keyPair.PublicKey.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("invalid Ed25519 public key")
		}

		jwk.Kty = "OKP"
		jwk.Alg = "EdDSA"
		jwk.Crv = "Ed25519"
		jwk.X = base64.RawURLEncoding.EncodeToString(edKey)

	case AlgorithmRS256:
		// RSA
		rsaKey, ok := keyPair.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("invalid RSA public key")
		}

		jwk.Kty = "RSA"
		jwk.Alg = "RS256"
		jwk.N = base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
		jwk.E = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes())

	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", keyPair.Algorithm)
	}

	return jwk, nil
}
