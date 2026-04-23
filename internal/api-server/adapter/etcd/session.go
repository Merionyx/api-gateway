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

const spanSessionRepo = "internal/api-server/adapter/etcd.SessionRepository"

var (
	// ErrSessionAlreadyExists is returned when Create targets an existing session key.
	ErrSessionAlreadyExists = errors.New("etcd: session already exists")

	// ErrSessionCASConflict is returned when ReplaceCAS loses the mod_revision compare (concurrent refresh).
	ErrSessionCASConflict = errors.New("etcd: session compare-and-swap lost")
)

// SessionRepository stores interactive OAuth session values at sessions/{session_id}.
type SessionRepository struct {
	client *clientv3.Client
	prefix string
}

// NewSessionRepository builds a repository. keyPrefix is the auth v1 root (same as APIKeyRepository).
func NewSessionRepository(client *clientv3.Client, keyPrefix string) *SessionRepository {
	p := strings.TrimSpace(keyPrefix)
	if p == "" {
		p = DefaultAuthEtcdKeyPrefix
	}
	p = strings.TrimRight(p, "/")
	return &SessionRepository{client: client, prefix: p}
}

func validateCanonicalSessionUUIDv4(id string) (string, error) {
	s := strings.TrimSpace(id)
	u, err := uuid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("etcd: session_id: %w", err)
	}
	if u.Version() != 4 {
		return "", errors.New("etcd: session_id must be uuid v4")
	}
	canon := u.String()
	if canon != s {
		return "", errors.New("etcd: session_id must be canonical lowercase uuid v4")
	}
	return canon, nil
}

func (r *SessionRepository) sessionKey(sessionID string) (string, error) {
	canon, err := validateCanonicalSessionUUIDv4(sessionID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/sessions/%s", r.prefix, canon), nil
}

// Get loads a session value and mod_revision for CAS (ADR 0001).
func (r *SessionRepository) Get(ctx context.Context, sessionID string) (kvvalue.SessionValue, int64, error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "Get"))
	var err error
	defer func() {
		if err != nil && !errors.Is(err, apierrors.ErrNotFound) {
			telemetry.MarkError(span, err)
		}
		span.End()
	}()

	key, err := r.sessionKey(sessionID)
	if err != nil {
		return kvvalue.SessionValue{}, 0, err
	}
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return kvvalue.SessionValue{}, 0, apierrors.JoinStore("etcd get session", err)
	}
	if len(resp.Kvs) == 0 {
		err = apierrors.ErrNotFound
		return kvvalue.SessionValue{}, 0, err
	}
	kv := resp.Kvs[0]
	var rec kvvalue.SessionValue
	rec, err = kvvalue.ParseSessionValueJSON(kv.Value)
	if err != nil {
		return kvvalue.SessionValue{}, 0, apierrors.JoinStore("parse session value", err)
	}
	return rec, kv.ModRevision, nil
}

// Put overwrites the session record (operator / migration).
func (r *SessionRepository) Put(ctx context.Context, sessionID string, v kvvalue.SessionValue) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "Put"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.sessionKey(sessionID)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalSessionValueJSON(v)
	if err != nil {
		return err
	}
	_, err = r.client.Put(ctx, key, string(raw))
	if err != nil {
		return apierrors.JoinStore("etcd put session", err)
	}
	return nil
}

// Create writes the session only if the key does not exist (post-callback first write).
func (r *SessionRepository) Create(ctx context.Context, sessionID string, v kvvalue.SessionValue) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "Create"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	key, err := r.sessionKey(sessionID)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalSessionValueJSON(v)
	if err != nil {
		return err
	}
	cond := clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	txn := r.client.Txn(ctx).If(cond).Then(clientv3.OpPut(key, string(raw)))
	resp, err := txn.Commit()
	if err != nil {
		return apierrors.JoinStore("etcd txn create session", err)
	}
	if !resp.Succeeded {
		return ErrSessionAlreadyExists
	}
	return nil
}

// ReplaceCAS updates the session only if mod_revision matches (refresh winner path).
func (r *SessionRepository) ReplaceCAS(ctx context.Context, sessionID string, v kvvalue.SessionValue, expectedModRevision int64) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "ReplaceCAS"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	if expectedModRevision <= 0 {
		return errors.New("etcd: ReplaceCAS expected_mod_revision must be > 0")
	}
	key, err := r.sessionKey(sessionID)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalSessionValueJSON(v)
	if err != nil {
		return err
	}
	cmp := clientv3.Compare(clientv3.ModRevision(key), "=", expectedModRevision)
	txn := r.client.Txn(ctx).If(cmp).Then(clientv3.OpPut(key, string(raw)))
	resp, err := txn.Commit()
	if err != nil {
		return apierrors.JoinStore("etcd txn replace session", err)
	}
	if !resp.Succeeded {
		return ErrSessionCASConflict
	}
	return nil
}
