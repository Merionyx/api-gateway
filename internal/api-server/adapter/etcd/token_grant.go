package etcd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const spanTokenGrantRepo = "internal/api-server/adapter/etcd.TokenGrantRepository"

// TokenGrantRepository stores per-token delegated permissions at token-grants/{jti}.
type TokenGrantRepository struct {
	client *clientv3.Client
	prefix string
}

// NewTokenGrantRepository builds a repository. keyPrefix is the auth v1 root (same as APIKeyRepository).
func NewTokenGrantRepository(client *clientv3.Client, keyPrefix string) *TokenGrantRepository {
	p := strings.TrimSpace(keyPrefix)
	if p == "" {
		p = DefaultAuthEtcdKeyPrefix
	}
	p = strings.TrimRight(p, "/")
	return &TokenGrantRepository{client: client, prefix: p}
}

func validateTokenJTI(jti string) (string, error) {
	s := strings.TrimSpace(jti)
	if s == "" {
		return "", errors.New("etcd: token jti is required")
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("etcd: token jti: %w", err)
	}
	canon := u.String()
	if canon != s {
		return "", errors.New("etcd: token jti must be canonical lowercase uuid")
	}
	return canon, nil
}

func (r *TokenGrantRepository) tokenGrantKey(jti string) (string, error) {
	canon, err := validateTokenJTI(jti)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/token-grants/%s", r.prefix, canon), nil
}

// Get returns token grant and mod_revision for CAS callers. ErrNotFound if missing.
func (r *TokenGrantRepository) Get(ctx context.Context, jti string) (kvvalue.TokenGrantValue, int64, error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanTokenGrantRepo, "Get"))
	var err error
	defer func() {
		if err != nil && !errors.Is(err, apierrors.ErrNotFound) {
			telemetry.MarkError(span, err)
		}
		span.End()
	}()

	key, err := r.tokenGrantKey(jti)
	if err != nil {
		return kvvalue.TokenGrantValue{}, 0, err
	}
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return kvvalue.TokenGrantValue{}, 0, apierrors.JoinStore("etcd get token grant", err)
	}
	if len(resp.Kvs) == 0 {
		err = apierrors.ErrNotFound
		return kvvalue.TokenGrantValue{}, 0, err
	}
	kv := resp.Kvs[0]
	rec, err := kvvalue.ParseTokenGrantValueJSON(kv.Value)
	if err != nil {
		return kvvalue.TokenGrantValue{}, 0, apierrors.JoinStore("parse token grant value", err)
	}
	return rec, kv.ModRevision, nil
}

// Put writes or overwrites token grant for jti.
func (r *TokenGrantRepository) Put(ctx context.Context, jti string, rec kvvalue.TokenGrantValue) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanTokenGrantRepo, "Put"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.tokenGrantKey(jti)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalTokenGrantValueJSON(rec)
	if err != nil {
		return err
	}
	_, err = r.client.Put(ctx, key, string(raw))
	if err != nil {
		return apierrors.JoinStore("etcd put token grant", err)
	}
	return nil
}

// Delete removes token grant for jti (best-effort cleanup).
func (r *TokenGrantRepository) Delete(ctx context.Context, jti string) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanTokenGrantRepo, "Delete"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.tokenGrantKey(jti)
	if err != nil {
		return err
	}
	_, err = r.client.Delete(ctx, key)
	if err != nil {
		return apierrors.JoinStore("etcd delete token grant", err)
	}
	return nil
}
