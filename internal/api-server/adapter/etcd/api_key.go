package etcd

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const spanAPIKeyRepo = "internal/api-server/adapter/etcd.APIKeyRepository"

// DefaultAuthEtcdKeyPrefix is the canonical auth v1 root (docs/etcd-auth-paths.md).
const DefaultAuthEtcdKeyPrefix = "/api-gateway/api-server/auth/v1"

var (
	// ErrBootstrapAPIKeyDisabled is returned when bootstrap is not allowed by config.
	ErrBootstrapAPIKeyDisabled = errors.New("etcd: api key bootstrap disabled by configuration")

	// ErrAPIKeyRecordExists is returned when bootstrap targets a key that already exists.
	ErrAPIKeyRecordExists = errors.New("etcd: api key record already exists")

	// ErrInvalidDigestHex is returned for malformed SHA-256 hex digest (path segment).
	ErrInvalidDigestHex = errors.New("etcd: invalid sha256 digest hex")
)

// APIKeyRepository persists API key metadata under .../api-keys/sha256/{digest_hex}.
type APIKeyRepository struct {
	client *clientv3.Client
	prefix string // normalized auth root without trailing slash
}

// NewAPIKeyRepository builds a repository. keyPrefix should be the auth v1 root; empty uses DefaultAuthEtcdKeyPrefix.
func NewAPIKeyRepository(client *clientv3.Client, keyPrefix string) *APIKeyRepository {
	p := strings.TrimSpace(keyPrefix)
	if p == "" {
		p = DefaultAuthEtcdKeyPrefix
	}
	p = strings.TrimRight(p, "/")
	return &APIKeyRepository{client: client, prefix: p}
}

// SHA256DigestHexFromSecret returns the 64-char lowercase hex SHA-256 of the secret (for etcd key path).
func SHA256DigestHexFromSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeDigestHexEqual compares two 64-char lowercase hex digests in constant time (after decode).
func ConstantTimeDigestHexEqual(a, b string) (bool, error) {
	da, err := decodeDigestHex(a)
	if err != nil {
		return false, err
	}
	db, err := decodeDigestHex(b)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(da, db) == 1, nil
}

func decodeDigestHex(s string) ([]byte, error) {
	if err := validateDigestHex(s); err != nil {
		return nil, err
	}
	return hex.DecodeString(s)
}

func validateDigestHex(digestHex string) error {
	if len(digestHex) != sha256.Size*2 {
		return fmt.Errorf("%w: want 64 hex chars", ErrInvalidDigestHex)
	}
	for i := 0; i < len(digestHex); i++ {
		c := digestHex[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return fmt.Errorf("%w: non-lower-hex at %d", ErrInvalidDigestHex, i)
		}
	}
	return nil
}

func (r *APIKeyRepository) keyForDigest(digestHex string) (string, error) {
	if err := validateDigestHex(digestHex); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/api-keys/sha256/%s", r.prefix, digestHex), nil
}

// Get returns the record and etcd mod_revision for CAS callers. ErrNotFound if missing.
func (r *APIKeyRepository) Get(ctx context.Context, digestHex string) (kvvalue.APIKeyValue, int64, error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIKeyRepo, "Get"))
	var err error
	defer func() {
		if err != nil && !errors.Is(err, apierrors.ErrNotFound) {
			telemetry.MarkError(span, err)
		}
		span.End()
	}()

	key, err := r.keyForDigest(digestHex)
	if err != nil {
		return kvvalue.APIKeyValue{}, 0, err
	}
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return kvvalue.APIKeyValue{}, 0, apierrors.JoinStore("etcd get api key", err)
	}
	if len(resp.Kvs) == 0 {
		err = apierrors.ErrNotFound
		return kvvalue.APIKeyValue{}, 0, err
	}
	kv := resp.Kvs[0]
	var rec kvvalue.APIKeyValue
	rec, err = kvvalue.ParseAPIKeyValueJSON(kv.Value)
	if err != nil {
		return kvvalue.APIKeyValue{}, 0, apierrors.JoinStore("parse api key value", err)
	}
	return rec, kv.ModRevision, nil
}

// Put writes or overwrites the api-key record (operator / migration use). Prefer BootstrapPutDevelopment for first key in dev.
func (r *APIKeyRepository) Put(ctx context.Context, digestHex string, rec kvvalue.APIKeyValue) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIKeyRepo, "Put"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.keyForDigest(digestHex)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalAPIKeyValueJSON(rec)
	if err != nil {
		return err
	}
	_, err = r.client.Put(ctx, key, string(raw))
	if err != nil {
		return apierrors.JoinStore("etcd put api key", err)
	}
	return nil
}

// BootstrapPutDevelopment creates the etcd record for secret's digest only if the key does not exist.
// allowed must be config.Auth.BootstrapAPIKeyAllowed(); otherwise ErrBootstrapAPIKeyDisabled.
func (r *APIKeyRepository) BootstrapPutDevelopment(ctx context.Context, allowed bool, secret string, rec kvvalue.APIKeyValue) (digestHex string, err error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIKeyRepo, "BootstrapPutDevelopment"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	if !allowed {
		return "", ErrBootstrapAPIKeyDisabled
	}
	if secret == "" {
		return "", fmt.Errorf("etcd: empty api key secret")
	}
	digestHex = SHA256DigestHexFromSecret(secret)
	key, err := r.keyForDigest(digestHex)
	if err != nil {
		return "", err
	}
	if rec.Algorithm == "" {
		rec.Algorithm = "sha256"
	}
	raw, err := kvvalue.MarshalAPIKeyValueJSON(rec)
	if err != nil {
		return "", err
	}

	cond := clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	txn := r.client.Txn(ctx).If(cond).Then(clientv3.OpPut(key, string(raw)))
	resp, err := txn.Commit()
	if err != nil {
		return "", apierrors.JoinStore("etcd txn bootstrap api key", err)
	}
	if !resp.Succeeded {
		return "", ErrAPIKeyRecordExists
	}
	return digestHex, nil
}
