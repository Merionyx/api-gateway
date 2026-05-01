package etcd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const spanLoginIntentRepo = "internal/api-server/adapter/etcd.LoginIntentRepository"

// LoginIntentRepository stores short-lived OIDC login context at login-intents/{intent_id} with a lease.
type LoginIntentRepository struct {
	client *clientv3.Client
	prefix string
}

// NewLoginIntentRepository builds a repository. keyPrefix is the auth v1 root (same as SessionRepository).
func NewLoginIntentRepository(client *clientv3.Client, keyPrefix string) *LoginIntentRepository {
	p := strings.TrimSpace(keyPrefix)
	if p == "" {
		p = DefaultAuthEtcdKeyPrefix
	}
	p = strings.TrimRight(p, "/")
	return &LoginIntentRepository{client: client, prefix: p}
}

func validateCanonicalIntentUUIDv4(id string) (string, error) {
	s := strings.TrimSpace(id)
	u, err := uuid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("etcd: intent_id: %w", err)
	}
	if u.Version() != 4 {
		return "", errors.New("etcd: intent_id must be uuid v4")
	}
	canon := u.String()
	if canon != s {
		return "", errors.New("etcd: intent_id must be canonical lowercase uuid v4")
	}
	return canon, nil
}

func (r *LoginIntentRepository) intentKey(intentID string) (string, error) {
	canon, err := validateCanonicalIntentUUIDv4(intentID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/login-intents/%s", r.prefix, canon), nil
}

// Create stores the login-intent value with an etcd lease. The key must not exist yet.
func (r *LoginIntentRepository) Create(ctx context.Context, intentID string, v kvvalue.LoginIntentValue, leaseTTL time.Duration) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanLoginIntentRepo, "Create"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.intentKey(intentID)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalLoginIntentValueJSON(v)
	if err != nil {
		return err
	}

	sec := int64(leaseTTL.Round(time.Second) / time.Second)
	if sec < 60 {
		sec = 60
	}
	if sec > 86400 {
		sec = 86400
	}

	grant, err := r.client.Grant(ctx, sec)
	if err != nil {
		return apierrors.JoinStore("etcd grant lease login-intent", err)
	}

	cond := clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	txn := r.client.Txn(ctx).If(cond).Then(clientv3.OpPut(key, string(raw), clientv3.WithLease(grant.ID)))
	resp, err := txn.Commit()
	if err != nil {
		return apierrors.JoinStore("etcd txn create login-intent", err)
	}
	if !resp.Succeeded {
		err = errors.New("etcd: login-intent key already exists")
		return err
	}
	return nil
}

// Get returns the login-intent value or apierrors.ErrNotFound.
func (r *LoginIntentRepository) Get(ctx context.Context, intentID string) (kvvalue.LoginIntentValue, error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanLoginIntentRepo, "Get"))
	var err error
	defer func() {
		if err != nil && !errors.Is(err, apierrors.ErrNotFound) {
			telemetry.MarkError(span, err)
		}
		span.End()
	}()

	key, err := r.intentKey(intentID)
	if err != nil {
		return kvvalue.LoginIntentValue{}, err
	}
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return kvvalue.LoginIntentValue{}, apierrors.JoinStore("etcd get login-intent", err)
	}
	if len(resp.Kvs) == 0 {
		err = apierrors.ErrNotFound
		return kvvalue.LoginIntentValue{}, err
	}
	var v kvvalue.LoginIntentValue
	v, err = kvvalue.ParseLoginIntentValueJSON(resp.Kvs[0].Value)
	if err != nil {
		return kvvalue.LoginIntentValue{}, apierrors.JoinStore("parse login-intent value", err)
	}
	return v, nil
}

// Delete removes the login-intent key (best-effort idempotent).
func (r *LoginIntentRepository) Delete(ctx context.Context, intentID string) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanLoginIntentRepo, "Delete"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.intentKey(intentID)
	if err != nil {
		return err
	}
	_, err = r.client.Delete(ctx, key)
	if err != nil {
		return apierrors.JoinStore("etcd delete login-intent", err)
	}
	return nil
}
