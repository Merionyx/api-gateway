package apierrors

import "errors"

// ErrNotFound indicates a missing domain resource (HTTP 404).
var ErrNotFound = errors.New("not found")

// ErrContractSyncerRejected marks a non-transient failure from Contract Syncer (invalid bundle, etc.); do not transport-retry.
var ErrContractSyncerRejected = errors.New("contract syncer rejected sync")
