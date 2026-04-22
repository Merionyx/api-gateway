package usecase

import (
	"context"
	"errors"

	"github.com/merionyx/api-gateway/internal/controller/metrics"
)

// Register-and-stream (outer loop) documented in
//
// afterRegisterAndStreamSession — thin FSM step after one leaderAPIServerStream.runAPIServerSession
// (backoff, exit, session end metric).
//
// «Leader change» (HA) is not modeled here: the orchestrator cancels RegisterAndStream through
// context.Cancel and restarts when it becomes the leader again.

// registerAndStreamStep — result of one iteration of the outer loop (after session attempt).
// sessionEnd field: non-empty value → one call [metrics.RecordAPIServerSessionEnd].
type registerAndStreamStep struct {
	// returnErr: value for return from [leaderAPIServerStream.registerAndStream] at endLoop.
	returnErr  error
	endLoop    bool
	sessionEnd string // metrics.SessionReasonCanceled, SessionReasonError, or "".
}

// afterRegisterAndStreamSession classifies the outcome, matching the previous order of checks
// (first cancel ctx, then nil, then context.Canceled, otherwise — repeat with backoff).
func afterRegisterAndStreamSession(ctx context.Context, sessErr error) registerAndStreamStep {
	if err := ctx.Err(); err != nil {
		return registerAndStreamStep{
			returnErr:  err,
			endLoop:    true,
			sessionEnd: metrics.SessionReasonCanceled,
		}
	}
	if sessErr == nil {
		return registerAndStreamStep{
			endLoop:    true,
			sessionEnd: "",
		}
	}
	if errors.Is(sessErr, context.Canceled) {
		return registerAndStreamStep{
			returnErr:  sessErr,
			endLoop:    true,
			sessionEnd: metrics.SessionReasonCanceled,
		}
	}
	return registerAndStreamStep{
		endLoop:    false,
		sessionEnd: metrics.SessionReasonError,
	}
}
