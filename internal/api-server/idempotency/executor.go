package idempotency

import (
	"context"
)

// Executor runs bundle-sync idempotency: same Idempotency-Key + body fingerprint → same HTTP outcome.
// Implementations: in-memory (single replica) or etcd (shared across API Server replicas).
type Executor interface {
	Execute(ctx context.Context, idempotencyKey, bodyHash string, fn func() (*HTTPResult, error)) (*HTTPResult, error)
}
