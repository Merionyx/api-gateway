package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/metrics"
)

type afterSessionExpect struct {
	endLoop     bool
	sessionEnd  string
	returnNil   bool
	checkCtxErr bool
	returnIs    error
}

func TestAfterRegisterAndStreamSession(t *testing.T) {
	t.Parallel()

	other := errors.New("dial failed")

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		sessErr error
		exp     afterSessionExpect
	}{
		{
			name:    "context done wins over nil session",
			ctx:     canceledCtx,
			sessErr: nil,
			exp: afterSessionExpect{endLoop: true, sessionEnd: metrics.SessionReasonCanceled, checkCtxErr: true},
		},
		{
			name:    "clean exit no metric",
			ctx:     context.Background(),
			sessErr: nil,
			exp:     afterSessionExpect{endLoop: true, sessionEnd: "", returnNil: true},
		},
		{
			name:    "session canceled with live parent ctx",
			ctx:     context.Background(),
			sessErr: context.Canceled,
			exp:     afterSessionExpect{endLoop: true, sessionEnd: metrics.SessionReasonCanceled, returnIs: context.Canceled},
		},
		{
			name:    "recoverable error retries",
			ctx:     context.Background(),
			sessErr: other,
			exp:     afterSessionExpect{endLoop: false, sessionEnd: metrics.SessionReasonError},
		},
		{
			name:    "context done with failing session",
			ctx:     canceledCtx,
			sessErr: fmt.Errorf("open StreamSnapshots: %w", other),
			exp:     afterSessionExpect{endLoop: true, sessionEnd: metrics.SessionReasonCanceled, checkCtxErr: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			step := afterRegisterAndStreamSession(tt.ctx, tt.sessErr)
			if step.endLoop != tt.exp.endLoop {
				t.Errorf("endLoop: got %v, want %v", step.endLoop, tt.exp.endLoop)
			}
			if step.sessionEnd != tt.exp.sessionEnd {
				t.Errorf("sessionEnd: got %q, want %q", step.sessionEnd, tt.exp.sessionEnd)
			}
			if tt.exp.returnNil {
				if step.returnErr != nil {
					t.Errorf("returnErr: want nil, got %v", step.returnErr)
				}
				return
			}
			if tt.exp.checkCtxErr {
				if err := tt.ctx.Err(); err == nil {
					t.Fatal("ctx should be done")
				} else if !errors.Is(step.returnErr, err) {
					t.Errorf("returnErr: want %v, got %v", err, step.returnErr)
				}
				return
			}
			if tt.exp.returnIs != nil && !errors.Is(step.returnErr, tt.exp.returnIs) {
				t.Errorf("returnErr: want is %v, got %v", tt.exp.returnIs, step.returnErr)
			}
		})
	}
}
