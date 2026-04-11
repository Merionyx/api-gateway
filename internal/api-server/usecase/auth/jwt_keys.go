package auth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func (uc *JWTUseCase) loadKeys() error {
	if err := os.MkdirAll(uc.keysDir, 0700); err != nil {
		return err
	}

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

		if filepath.Ext(file.Name()) == ".key" {
			keyPath := filepath.Join(uc.keysDir, file.Name())

			keyPair, err := uc.loadKeyPair(keyPath)
			if err != nil {
				slog.Warn("jwt: skip key file", "file", file.Name(), "error", err)
				continue
			}

			uc.signingKeys[keyPair.Kid] = keyPair

			if keyPair.CreatedAt.After(newestTime) {
				newestTime = keyPair.CreatedAt
				newestKey = keyPair
			}
		}
	}

	if newestKey != nil {
		uc.activeKeyID = newestKey.Kid
		uc.activeKeyAlg = newestKey.Algorithm
		slog.Info("jwt: active signing key", "kid", uc.activeKeyID, "alg", uc.activeKeyAlg)
	}

	return nil
}

func (uc *JWTUseCase) loadKeyPair(keyPath string) (*KeyPair, error) {
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

	kid := filepath.Base(keyPath)
	kid = kid[:len(kid)-len(filepath.Ext(kid))]

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

func (uc *JWTUseCase) generateDefaultKey() error {
	kid := fmt.Sprintf("api-server-key-%s", time.Now().Format("2006-01-02"))
	keyPath := filepath.Join(uc.keysDir, kid+".key")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		return err
	}

	uc.signingKeys[kid] = &KeyPair{
		Kid:        kid,
		Algorithm:  AlgorithmEdDSA,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  time.Now(),
	}

	uc.activeKeyID = kid
	uc.activeKeyAlg = AlgorithmEdDSA

	slog.Info("jwt: generated default EdDSA key", "kid", kid)

	return nil
}
