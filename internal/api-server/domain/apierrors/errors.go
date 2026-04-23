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

// OIDC login (HTTP)
var (
	// ErrOIDCNotConfigured means no OIDC providers are configured (HTTP 400).
	ErrOIDCNotConfigured = errors.New("oidc login not configured")

	// ErrOIDCUnknownProvider means provider_id does not match a configured provider (HTTP 400).
	ErrOIDCUnknownProvider = errors.New("unknown oidc provider_id")

	// ErrOIDCRedirectNotAllowlisted means redirect_uri is not on the provider allowlist (HTTP 400).
	ErrOIDCRedirectNotAllowlisted = errors.New("redirect_uri not allowlisted")

	// ErrSessionRefreshConflict means etcd CAS lost during refresh (HTTP 409, ADR 0001).
	ErrSessionRefreshConflict = errors.New("session refresh state conflict")

	// ErrSessionAuthFailed means invalid or unknown refresh / session (HTTP 401).
	ErrSessionAuthFailed = errors.New("session authentication failed")

	// ErrGitHubLoginDenied means the user failed GitHub org/team policy at callback (HTTP 403).
	ErrGitHubLoginDenied = errors.New("github login denied by org or team policy")
)
