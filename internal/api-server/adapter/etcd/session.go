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

func validateRefreshVerifierHex(v string) error {
	s := strings.TrimSpace(v)
	if len(s) != 64 {
		return errors.New("etcd: refresh verifier must be 64 lowercase hex chars")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return errors.New("etcd: refresh verifier must be lowercase hex")
	}
	return nil
}

func (r *SessionRepository) refreshVerifierIndexKey(verifier string) (string, error) {
	if err := validateRefreshVerifierHex(verifier); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/session-refresh-verifiers/%s", r.prefix, strings.TrimSpace(verifier)), nil
}

// Create writes the session and refresh-verifier index atomically (post-callback first write).
func (r *SessionRepository) Create(ctx context.Context, sessionID string, v kvvalue.SessionValue) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "Create"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	if strings.TrimSpace(v.OurRefreshVerifier) == "" {
		return errors.New("etcd: session OurRefreshVerifier is required")
	}
	canonID, err := validateCanonicalSessionUUIDv4(sessionID)
	if err != nil {
		return err
	}
	key, err := r.sessionKey(sessionID)
	if err != nil {
		return err
	}
	idxKey, err := r.refreshVerifierIndexKey(v.OurRefreshVerifier)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalSessionValueJSON(v)
	if err != nil {
		return err
	}
	condS := clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
	condI := clientv3.Compare(clientv3.CreateRevision(idxKey), "=", 0)
	txn := r.client.Txn(ctx).If(condS, condI).Then(
		clientv3.OpPut(key, string(raw)),
		clientv3.OpPut(idxKey, canonID),
	)
	resp, err := txn.Commit()
	if err != nil {
		return apierrors.JoinStore("etcd txn create session", err)
	}
	if !resp.Succeeded {
		return ErrSessionAlreadyExists
	}
	return nil
}

// GetSessionIDByRefreshVerifier resolves sessions/{id} via session-refresh-verifiers/{sha256_hex}.
func (r *SessionRepository) GetSessionIDByRefreshVerifier(ctx context.Context, verifier string) (string, error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "GetSessionIDByRefreshVerifier"))
	var err error
	defer func() {
		if err != nil && !errors.Is(err, apierrors.ErrNotFound) {
			telemetry.MarkError(span, err)
		}
		span.End()
	}()

	idxKey, err := r.refreshVerifierIndexKey(verifier)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Get(ctx, idxKey)
	if err != nil {
		return "", apierrors.JoinStore("etcd get refresh index", err)
	}
	if len(resp.Kvs) == 0 {
		err = apierrors.ErrNotFound
		return "", err
	}
	sid := strings.TrimSpace(string(resp.Kvs[0].Value))
	if _, err := validateCanonicalSessionUUIDv4(sid); err != nil {
		return "", fmt.Errorf("etcd: corrupt refresh index value: %w", err)
	}
	return sid, nil
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

// ReplaceCASWithRefreshIndex updates the session and rotates the refresh-verifier index in one txn (ADR 0001).
func (r *SessionRepository) ReplaceCASWithRefreshIndex(ctx context.Context, sessionID, oldVerifier, newVerifier string, v kvvalue.SessionValue, expectedModRevision int64) error {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanSessionRepo, "ReplaceCASWithRefreshIndex"))
	var err error
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()

	if expectedModRevision <= 0 {
		return errors.New("etcd: ReplaceCASWithRefreshIndex expected_mod_revision must be > 0")
	}
	if strings.TrimSpace(oldVerifier) == "" || strings.TrimSpace(newVerifier) == "" {
		return errors.New("etcd: old and new refresh verifiers are required")
	}
	if oldVerifier == newVerifier {
		return errors.New("etcd: refresh verifiers must differ on rotation")
	}
	canonID, err := validateCanonicalSessionUUIDv4(sessionID)
	if err != nil {
		return err
	}
	key, err := r.sessionKey(sessionID)
	if err != nil {
		return err
	}
	oldIdx, err := r.refreshVerifierIndexKey(oldVerifier)
	if err != nil {
		return err
	}
	newIdx, err := r.refreshVerifierIndexKey(newVerifier)
	if err != nil {
		return err
	}
	raw, err := kvvalue.MarshalSessionValueJSON(v)
	if err != nil {
		return err
	}
	cmp := clientv3.Compare(clientv3.ModRevision(key), "=", expectedModRevision)
	ops := []clientv3.Op{
		clientv3.OpPut(key, string(raw)),
		clientv3.OpDelete(oldIdx),
		clientv3.OpPut(newIdx, canonID),
	}
	resp, err := r.client.Txn(ctx).If(cmp).Then(ops...).Commit()
	if err != nil {
		return apierrors.JoinStore("etcd txn replace session+refresh index", err)
	}
	if !resp.Succeeded {
		return ErrSessionCASConflict
	}
	return nil
}
