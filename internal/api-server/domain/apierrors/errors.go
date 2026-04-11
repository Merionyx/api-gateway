// Package apierrors defines sentinel errors for the API Server domain.
// Use errors.Is at HTTP/gRPC boundaries; wrap with JoinStore / JoinContractSyncer or fmt.Errorf("…: %w", sentinel).
package apierrors

import "errors"

// Common registry / bundle reads
var (
	// ErrNotFound indicates a missing domain resource (HTTP 404, gRPC NotFound).
	ErrNotFound = errors.New("not found")

	// ErrInvalidInput indicates client request validation failure (HTTP 400, gRPC InvalidArgument).
	ErrInvalidInput = errors.New("invalid input")
)

// Persistence (etcd)
var (
	// ErrStoreAccess indicates etcd or snapshot store I/O failure (HTTP 503, gRPC Unavailable).
	ErrStoreAccess = errors.New("store access failed")
)

// Contract Syncer
var (
	// ErrContractSyncerRejected marks a non-transient business failure from Contract Syncer (invalid bundle, etc.);
	// do not transport-retry (HTTP 400, gRPC InvalidArgument).
	ErrContractSyncerRejected = errors.New("contract syncer rejected sync")

	// ErrContractSyncerUnavailable indicates dial/RPC/transport failure to Contract Syncer (HTTP 502, gRPC Unavailable).
	ErrContractSyncerUnavailable = errors.New("contract syncer unavailable")
)

// JWT issuance
var (
	// ErrNoActiveSigningKey means no usable signing key is configured (HTTP 503, gRPC Unavailable).
	ErrNoActiveSigningKey = errors.New("no active signing key")

	// ErrUnsupportedSigningAlgorithm means the active key uses an algorithm we cannot sign with (HTTP 500, gRPC Internal).
	ErrUnsupportedSigningAlgorithm = errors.New("unsupported signing algorithm")

	// ErrSigningOperationFailed wraps crypto/signing failures after key selection (HTTP 500, gRPC Internal).
	ErrSigningOperationFailed = errors.New("signing operation failed")
)
