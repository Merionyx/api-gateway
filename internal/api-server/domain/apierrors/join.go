package apierrors

import (
	"errors"
	"fmt"
)

// JoinStore annotates err with ErrStoreAccess for errors.Is in handlers.
func JoinStore(op string, err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrStoreAccess, fmt.Errorf("%s: %w", op, err))
}

// JoinContractSyncer annotates transport/upstream failures (not ErrContractSyncerRejected).
func JoinContractSyncer(op string, err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrContractSyncerUnavailable, fmt.Errorf("%s: %w", op, err))
}
