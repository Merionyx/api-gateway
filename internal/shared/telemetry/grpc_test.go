package telemetry

import (
	"context"
	"testing"
)

func TestExtractIncomingGRPC_EmptyMD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := ExtractIncomingGRPC(ctx, nil); got != ctx {
		t.Fatal("empty md: expect same context")
	}
}

func TestOutgoingContextWithTrace_DoesNotPanic(t *testing.T) {
	t.Parallel()
	sh, err := Init(context.Background(), BuildConfig("t", FileBlock{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sh(context.Background()) })
	ctx, span := Start(context.Background(), "x")
	defer span.End()
	_ = OutgoingContextWithTrace(ctx)
}
