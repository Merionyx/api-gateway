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
	"slices"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// readKeyPairFromFile loads a single PEM private key and builds a KeyPair (kid from basename without .key).
func readKeyPairFromFile(keyPath string) (*KeyPair, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("%w: read key file: %w", apierrors.ErrInvalidInput, err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block in key file", apierrors.ErrInvalidInput)
	}

	var privateKey crypto.PrivateKey
	var publicKey crypto.PublicKey
	var algorithm string

	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: parse PKCS#8 private key: %w", apierrors.ErrInvalidInput, err)
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
			return nil, fmt.Errorf("%w: unsupported private key type %T", apierrors.ErrInvalidInput, k)
		}

	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: parse PKCS#1 private key: %w", apierrors.ErrInvalidInput, err)
		}
		privateKey = key
		publicKey = &key.PublicKey
		algorithm = AlgorithmRS256

	default:
		return nil, fmt.Errorf("%w: unsupported PEM block type %q", apierrors.ErrInvalidInput, block.Type)
	}

	kid := filepath.Base(keyPath)
	kid = kid[:len(kid)-len(filepath.Ext(kid))]

	fileInfo, err := os.Stat(keyPath)
	if err != nil {
		return nil, fmt.Errorf("%w: stat key file: %w", apierrors.ErrInvalidInput, err)
	}

	return &KeyPair{
		Kid:        kid,
		Algorithm:  algorithm,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  fileInfo.ModTime(),
	}, nil
}

// loadKeyDirectory loads all *.key PEM files from dir and picks the newest as active.
// It does not create the directory: on Kubernetes the mount is read-only and the default
// edge path keys_dir/edge may not exist as a sub-mount (use jwt.edge_keys_dir or Helm projected volumes).
func loadKeyDirectory(keysDir string) (map[string]*KeyPair, string, string, error) {
	fi, err := os.Stat(keysDir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*KeyPair), "", "", nil
		}
		return nil, "", "", err
	}
	if !fi.IsDir() {
		return nil, "", "", fmt.Errorf("%w: jwt keys path %q is not a directory", apierrors.ErrInvalidInput, keysDir)
	}

	files, err := os.ReadDir(keysDir)
	if err != nil {
		return nil, "", "", err
	}

	out := make(map[string]*KeyPair)
	var newestKey *KeyPair
	var newestTime time.Time

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) != ".key" {
			continue
		}
		keyPath := filepath.Join(keysDir, file.Name())
		keyPair, err := readKeyPairFromFile(keyPath)
		if err != nil {
			slog.Warn("jwt: skip key file", "dir", keysDir, "file", file.Name(), "error", err)
			continue
		}
		out[keyPair.Kid] = keyPair
		if keyPair.CreatedAt.After(newestTime) {
			newestTime = keyPair.CreatedAt
			newestKey = keyPair
		}
	}

	activeID, activeAlg := "", ""
	if newestKey != nil {
		activeID = newestKey.Kid
		activeAlg = newestKey.Algorithm
		slog.Info("jwt: newest key by mtime (default signing candidate)", "dir", keysDir, "kid", activeID, "alg", activeAlg)
	}
	return out, activeID, activeAlg, nil
}

func resolveSigningKeyID(profile string, keys map[string]*KeyPair, preferredKid, newestKid string) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("%w: jwt %s: no signing keys loaded", apierrors.ErrInvalidInput, profile)
	}
	if kid := strings.TrimSpace(preferredKid); kid != "" {
		if _, ok := keys[kid]; !ok {
			list := make([]string, 0, len(keys))
			for k := range keys {
				list = append(list, k)
			}
			slices.Sort(list)
			return "", fmt.Errorf("%w: jwt %s: signing kid %q not found (have: %s)", apierrors.ErrInvalidInput, profile, kid, strings.Join(list, ", "))
		}
		slog.Info("jwt: signing kid", "profile", profile, "kid", kid, "source", "config")
		return kid, nil
	}
	if newestKid == "" {
		return "", fmt.Errorf("%w: jwt %s: no signing key", apierrors.ErrInvalidInput, profile)
	}
	slog.Info("jwt: signing kid", "profile", profile, "kid", newestKid, "source", "newest_mtime")
	return newestKid, nil
}

// generateDefaultEd25519InDir writes a new Ed25519 key under keysDir and returns the loaded KeyPair.
func generateDefaultEd25519InDir(keysDir, kidPrefix string) (*KeyPair, error) {
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, err
	}
	kid := fmt.Sprintf("%s-%s", kidPrefix, time.Now().Format("2006-01-02-150405"))
	keyPath := filepath.Join(keysDir, kid+".key")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("%w: generate Ed25519 key: %w", apierrors.ErrSigningOperationFailed, err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal PKCS#8 private key: %w", apierrors.ErrSigningOperationFailed, err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		return nil, fmt.Errorf("%w: write generated key file: %w", apierrors.ErrSigningOperationFailed, err)
	}

	kp := &KeyPair{
		Kid:        kid,
		Algorithm:  AlgorithmEdDSA,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		CreatedAt:  time.Now(),
	}
	slog.Info("jwt: generated default EdDSA key", "dir", keysDir, "kid", kid)
	return kp, nil
}
