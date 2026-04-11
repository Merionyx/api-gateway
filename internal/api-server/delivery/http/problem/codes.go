package problem

// Stable problem codes (field `code` in Problem JSON) for i18n keys, e.g. "problem.INVALID_JSON_BODY".
// Titles and Detail* strings are default English for debugging / optional UI fallback — prefer translating by `code`.
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

	CodeNotFound                    = "NOT_FOUND"
	CodeInvalidInput                = "INVALID_INPUT"
	CodeContractSyncerRejected      = "CONTRACT_SYNCER_REJECTED"
	CodeNoActiveSigningKey          = "NO_ACTIVE_SIGNING_KEY"
	CodeUnsupportedSigningAlgorithm = "UNSUPPORTED_SIGNING_ALGORITHM"
	CodeSigningOperationFailed      = "SIGNING_OPERATION_FAILED"
	CodeStoreUnavailable            = "STORE_UNAVAILABLE"
	CodeContractSyncerUnavailable   = "CONTRACT_SYNCER_UNAVAILABLE"
	CodeInternalError               = "INTERNAL_ERROR"
	CodeContractSyncPipelineFailed  = "CONTRACT_SYNC_PIPELINE_FAILED"
)

// Default English detail lines (optional display; primary UI should use Code for i18n).
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

	DetailNotFound                    = "The requested resource was not found."
	DetailInvalidInput                = "The request parameters are not valid."
	DetailContractSyncerRejected      = "The contract syncer rejected this request."
	DetailNoActiveSigningKey          = "No active JWT signing key is configured."
	DetailUnsupportedSigningAlgorithm = "The configured signing algorithm is not supported."
	DetailSigningOperationFailed      = "Signing the token failed."
	DetailStoreUnavailable            = "Required storage is temporarily unavailable."
	DetailContractSyncerUnavailable   = "The contract sync service is temporarily unavailable."
	DetailInternalError               = "An unexpected error occurred."
	DetailContractSyncPipelineFailed  = "The contract sync request could not be completed."
)

// TypeURI returns RFC 7807 `type` — stable URI with fragment equal to `code`.
func TypeURI(code string) string {
	return ProblemsDocBase + "#" + code
}
