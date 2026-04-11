package errmapping

import (
	"errors"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// DetailContractSyncRejected returns client-facing detail for CONTRACT_SYNCER_REJECTED.
func DetailContractSyncRejected(err error) string {
	if err == nil || !errors.Is(err, apierrors.ErrContractSyncerRejected) {
		return DetailContractSyncerRejected
	}
	prefix := apierrors.ErrContractSyncerRejected.Error()
	full := err.Error()
	for _, line := range strings.Split(full, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				return rest
			}
			continue
		}
		return line
	}
	return DetailContractSyncerRejected
}
