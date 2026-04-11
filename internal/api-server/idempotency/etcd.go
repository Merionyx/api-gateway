package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sync/singleflight"
)

var _ Executor = (*EtcdStore)(nil)

// EtcdStore persists idempotent HTTP outcomes under a key prefix (lease TTL = eviction).
type EtcdStore struct {
	client *clientv3.Client
	prefix string
	ttl    time.Duration
	group  singleflight.Group
}

type persistedRecord struct {
	BodyHash string      `json:"body_hash"`
	Result   *HTTPResult `json:"result"`
}

// NewEtcdStore creates a shared idempotency store. resolvedPrefix is typically idempotency.ResolveKeyPrefix(...).
func NewEtcdStore(client *clientv3.Client, resolvedPrefix string, ttl time.Duration) *EtcdStore {
	if client == nil {
		panic("idempotency: nil etcd client")
	}
	p := strings.TrimSuffix(strings.TrimSpace(resolvedPrefix), "/")
	if p == "" {
		p = "/api-gateway/api-server/idempotency/v1"
	}
	return &EtcdStore{
		client: client,
		prefix: p + "/keys/",
		ttl:    ttl,
	}
}

func idempotencyKeyFingerprint(idempotencyKey string) string {
	sum := sha256.Sum256([]byte(idempotencyKey))
	return hex.EncodeToString(sum[:])
}

func (e *EtcdStore) etcdKey(idempotencyKey string) string {
	return e.prefix + idempotencyKeyFingerprint(idempotencyKey)
}

// Execute implements Executor. Concurrent requests with the same Idempotency-Key are serialized (singleflight per key).
func (e *EtcdStore) Execute(ctx context.Context, idempotencyKey, bodyHash string, fn func() (*HTTPResult, error)) (*HTTPResult, error) {
	if idempotencyKey == "" || bodyHash == "" {
		return fn()
	}
	fk := idempotencyKeyFingerprint(idempotencyKey)
	v, err, _ := e.group.Do(fk, func() (interface{}, error) {
		return e.executeOnce(ctx, idempotencyKey, bodyHash, fn)
	})
	if err != nil {
		return nil, err
	}
	return v.(*HTTPResult), nil
}

func (e *EtcdStore) executeOnce(ctx context.Context, idempotencyKey, bodyHash string, fn func() (*HTTPResult, error)) (*HTTPResult, error) {
	k := e.etcdKey(idempotencyKey)
	gresp, err := e.client.Get(ctx, k)
	if err != nil {
		return nil, apierrors.JoinStore("idempotency get", err)
	}
	if len(gresp.Kvs) > 0 {
		var rec persistedRecord
		if err := json.Unmarshal(gresp.Kvs[0].Value, &rec); err != nil {
			return nil, err
		}
		if rec.BodyHash != bodyHash {
			return nil, ErrConflict
		}
		if rec.Result != nil {
			return cloneResult(rec.Result), nil
		}
	}

	res, err := fn()
	if err != nil {
		return nil, err
	}
	rec := persistedRecord{BodyHash: bodyHash, Result: cloneResult(res)}
	data, err := json.Marshal(&rec)
	if err != nil {
		return nil, err
	}

	ttl := e.ttl
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	ttlSec := int64(ttl / time.Second)
	if ttlSec < 5 {
		ttlSec = 5 // etcd minimum lease TTL (seconds), typical default
	}

	lease, err := e.client.Grant(ctx, ttlSec)
	if err != nil {
		return nil, apierrors.JoinStore("idempotency lease", err)
	}
	_, err = e.client.Put(ctx, k, string(data), clientv3.WithLease(lease.ID))
	if err != nil {
		return nil, apierrors.JoinStore("idempotency put", err)
	}
	return cloneResult(res), nil
}
