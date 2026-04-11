package problem

import (
	"errors"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// DetailContractSyncRejected returns client-facing detail: upstream/business reason when present, else neutral default.
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
		// Non-sentinel line (e.g. wrapped transport message) — first such line is a useful hint.
		return line
	}
	return DetailContractSyncerRejected
}
