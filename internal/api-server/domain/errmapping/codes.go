// Package errmapping is the single source of truth for domain error → HTTP status + gRPC code + Problem code/detail.
package errmapping

// Problem codes and default English details for domain-mapped errors (i18n keys: problem.<CODE>).
const (
	CodeNotFound                    = "NOT_FOUND"
	CodeInvalidInput                = "INVALID_INPUT"
	CodeContractSyncerRejected      = "CONTRACT_SYNCER_REJECTED"
	CodeNoActiveSigningKey          = "NO_ACTIVE_SIGNING_KEY"
	CodeUnsupportedSigningAlgorithm = "UNSUPPORTED_SIGNING_ALGORITHM"
	CodeSigningOperationFailed      = "SIGNING_OPERATION_FAILED"
	CodeStoreUnavailable            = "STORE_UNAVAILABLE"
	CodeContractSyncerUnavailable   = "CONTRACT_SYNCER_UNAVAILABLE"
	CodeSessionRefreshConflict      = "REFRESH_STATE_CONFLICT"
	CodeSessionAuthFailed           = "SESSION_AUTH_FAILED"
	CodeInternalError               = "INTERNAL_ERROR"

	CodeContractSyncPipelineFailed = "CONTRACT_SYNC_PIPELINE_FAILED"
)

// Default English detail strings (Problem.detail and stable gRPC status message when mapped).
const (
	DetailNotFound                    = "The requested resource was not found."
	DetailInvalidInput                = "The request parameters are not valid."
	DetailContractSyncerRejected      = "The contract syncer rejected this request."
	DetailNoActiveSigningKey          = "No active JWT signing key is configured."
	DetailUnsupportedSigningAlgorithm = "The configured signing algorithm is not supported."
	DetailSigningOperationFailed      = "Signing the token failed."
	DetailStoreUnavailable            = "Required storage is temporarily unavailable."
	DetailContractSyncerUnavailable   = "The contract sync service is temporarily unavailable."
	DetailSessionRefreshConflict      = "Session state changed concurrently; retry with backoff or use the token pair from a successful refresh."
	DetailSessionAuthFailed           = "Refresh token is invalid, expired, or revoked."
	DetailInternalError               = "An unexpected error occurred."
	DetailContractSyncPipelineFailed  = "The contract sync request could not be completed."
)
