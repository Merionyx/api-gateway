// Package idempotency provides in-memory idempotency for HTTP mutations (single-process; HA requires shared store).
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

var _ Executor = (*Store)(nil)

// ErrConflict means the Idempotency-Key was already bound to a different request body.
var ErrConflict = errors.New("idempotency key reused with different request body")

// HTTPResult is a serialized HTTP response to replay for duplicate requests.
type HTTPResult struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

// Store is the in-process idempotency cache (single replica).
type Store struct {
	mu      sync.Mutex
	entries map[string]*entry
	ttl     time.Duration
}

type entry struct {
	bodyHash string
	created  time.Time

	done   bool
	result *HTTPResult
	fnErr  error

	wait chan struct{}
}

// NewStore creates an idempotency store. ttl is how long completed entries (success or failure) are retained.
func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{
		entries: make(map[string]*entry),
		ttl:     ttl,
	}
}

// HashBundleSyncRequest returns a stable fingerprint of the JSON body for POST /bundles/sync.
func HashBundleSyncRequest(req apiserver.BundleSyncRequest) string {
	b, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Execute implements Executor.
func (s *Store) Execute(ctx context.Context, idempotencyKey, bodyHash string, fn func() (*HTTPResult, error)) (*HTTPResult, error) {
	if idempotencyKey == "" || bodyHash == "" {
		return fn()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		now := time.Now()
		s.mu.Lock()
		s.evictExpiredLocked(now)

		e, ok := s.entries[idempotencyKey]
		if ok {
			if e.bodyHash != bodyHash {
				s.mu.Unlock()
				return nil, ErrConflict
			}
			if e.done {
				if e.fnErr != nil {
					err := e.fnErr
					s.mu.Unlock()
					return nil, err
				}
				r := cloneResult(e.result)
				s.mu.Unlock()
				return r, nil
			}
			ch := e.wait
			s.mu.Unlock()
			select {
			case <-ch:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}

		ne := &entry{
			bodyHash: bodyHash,
			created:  now,
			wait:     make(chan struct{}),
		}
		s.entries[idempotencyKey] = ne
		s.mu.Unlock()

		res, err := fn()
		s.mu.Lock()
		ne.done = true
		ne.fnErr = err
		if err == nil {
			ne.result = cloneResult(res)
		}
		close(ne.wait)
		s.mu.Unlock()

		if err != nil {
			return nil, err
		}
		return cloneResult(res), nil
	}
}

func (s *Store) evictExpiredLocked(now time.Time) {
	for k, e := range s.entries {
		if !e.done {
			continue
		}
		if now.Sub(e.created) > s.ttl {
			delete(s.entries, k)
		}
	}
}

func cloneResult(r *HTTPResult) *HTTPResult {
	if r == nil {
		return nil
	}
	out := *r
	if r.Body != nil {
		out.Body = append([]byte(nil), r.Body...)
	}
	return &out
}
