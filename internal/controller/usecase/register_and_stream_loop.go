package usecase

import (
	"context"
	"errors"

	"github.com/merionyx/api-gateway/internal/controller/metrics"
)

// Register-and-stream (внешний цикл) задокументирован в
// docs/refactor/api-server-sync-register-and-stream.md.
//
// afterRegisterAndStreamSession — тонкий шаг FSM после одного leaderAPIServerStream.runAPIServerSession
// (backoff, выход, метрика конца сессии).
//
// «Смена лидера» (HA) здесь не моделируется: оркестратор снимает RegisterAndStream через
// context.Cancel и снова запускает, когда снова стали лидером.

// registerAndStreamStep — результат одной итерации внешнего цикла (после попытки сессии).
// Поле sessionEnd: непустое значение → один вызов [metrics.RecordAPIServerSessionEnd].
type registerAndStreamStep struct {
	// returnErr: значение для return из [leaderAPIServerStream.registerAndStream] при endLoop.
	returnErr  error
	endLoop    bool
	sessionEnd string // metrics.SessionReasonCanceled, SessionReasonError, or "".
}

// afterRegisterAndStreamSession классифицирует исход, совпадая с прежним порядком проверок
// (сначала отмена ctx, затем nil, затем context.Canceled, иначе — повтор с backoff).
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