package problem

import "github.com/merionyx/api-gateway/internal/api-server/domain/errmapping"

// Stable problem codes for handlers (validation, path params). Domain-mapped codes are re-exported from errmapping.
const (
	ProblemsDocBase = "https://gateway.merionyx.com/problems/v1"

	CodeInvalidJSONBody             = "INVALID_JSON_BODY"
	CodeSyncBundleParamsRequired    = "SYNC_BUNDLE_PARAMS_REQUIRED"
	CodeInvalidBundleKeyPath        = "INVALID_BUNDLE_KEY_PATH"
	CodeInvalidContractNamePath     = "INVALID_CONTRACT_NAME_PATH"
	CodeContractNotInBundle         = "CONTRACT_NOT_IN_BUNDLE"
	CodeControllerNotFound          = "CONTROLLER_NOT_FOUND"
	CodeControllerHeartbeatNotFound = "CONTROLLER_HEARTBEAT_NOT_FOUND"

	CodeTokenAppIDRequired        = "TOKEN_APP_ID_REQUIRED"
	CodeTokenEnvironmentsRequired = "TOKEN_ENVIRONMENTS_REQUIRED"
	CodeTokenEnvironmentEmpty     = "TOKEN_ENVIRONMENT_EMPTY"
	CodeTokenExpiresAtPast        = "TOKEN_EXPIRES_AT_PAST"

	CodeExportRepositoryRefRequired = "EXPORT_REPOSITORY_REF_REQUIRED"
)

// Domain / pipeline codes — single source: errmapping.
const (
	CodeNotFound                    = errmapping.CodeNotFound
	CodeInvalidInput                = errmapping.CodeInvalidInput
	CodeContractSyncerRejected      = errmapping.CodeContractSyncerRejected
	CodeNoActiveSigningKey          = errmapping.CodeNoActiveSigningKey
	CodeUnsupportedSigningAlgorithm = errmapping.CodeUnsupportedSigningAlgorithm
	CodeSigningOperationFailed      = errmapping.CodeSigningOperationFailed
	CodeStoreUnavailable            = errmapping.CodeStoreUnavailable
	CodeContractSyncerUnavailable   = errmapping.CodeContractSyncerUnavailable
	CodeInternalError               = errmapping.CodeInternalError
	CodeContractSyncPipelineFailed  = errmapping.CodeContractSyncPipelineFailed
)

// Default English detail lines for handler-level validation (i18n fallback).
const (
	DetailInvalidJSONBody = "The request body could not be read as valid JSON."

	DetailSyncBundleParamsRequired    = "Fields repository, ref, and bundle are required."
	DetailInvalidBundleKeyPath        = "The bundle_key path parameter is not valid."
	DetailInvalidContractNamePath     = "The contract_name path parameter is not valid."
	DetailContractNotInBundle         = "No contract with this name exists in the bundle."
	DetailControllerNotFound          = "No controller with this identifier exists."
	DetailControllerHeartbeatNotFound = "No heartbeat record exists for this controller."

	DetailTokenAppIDRequired        = "Field app_id is required."
	DetailTokenEnvironmentsRequired = "At least one environment is required."
	DetailTokenEnvironmentEmpty     = "Each environment must be a non-empty string."
	DetailTokenExpiresAtPast        = "expires_at must be in the future."

	DetailExportRepositoryRefRequired = "Fields repository and ref are required."
)

// Domain detail strings — re-export for callers that reference problem.DetailNotFound etc.
const (
	DetailNotFound                    = errmapping.DetailNotFound
	DetailInvalidInput                = errmapping.DetailInvalidInput
	DetailContractSyncerRejected      = errmapping.DetailContractSyncerRejected
	DetailNoActiveSigningKey          = errmapping.DetailNoActiveSigningKey
	DetailUnsupportedSigningAlgorithm = errmapping.DetailUnsupportedSigningAlgorithm
	DetailSigningOperationFailed      = errmapping.DetailSigningOperationFailed
	DetailStoreUnavailable            = errmapping.DetailStoreUnavailable
	DetailContractSyncerUnavailable   = errmapping.DetailContractSyncerUnavailable
	DetailInternalError               = errmapping.DetailInternalError
	DetailContractSyncPipelineFailed  = errmapping.DetailContractSyncPipelineFailed
)

// TypeURI returns RFC 7807 `type` — stable URI with fragment equal to `code`.
func TypeURI(code string) string {
	return ProblemsDocBase + "#" + code
}
