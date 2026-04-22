package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// mdCarrier implements propagation.TextMapCarrier for gRPC metadata.
type mdCarrier struct {
	md *metadata.MD
}

func (c *mdCarrier) Get(key string) string {
	vs := c.md.Get(key)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func (c *mdCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

func (c *mdCarrier) Keys() []string {
	out := make([]string, 0, len(*c.md))
	for k := range *c.md {
		out = append(out, k)
	}
	return out
}

// ExtractIncomingGRPC returns ctx with the remote W3C trace context (if any) from
// gRPC [metadata]. Call at the start of a server handler **before** [Start]:
//
//	md, _ := metadata.FromIncomingContext(ctx)
//	ctx = telemetry.ExtractIncomingGRPC(ctx, md)
//	ctx, span := telemetry.Start(ctx, telemetry.SpanName(...))
//	defer span.End()
func ExtractIncomingGRPC(ctx context.Context, md metadata.MD) context.Context {
	if len(md) == 0 {
		return ctx
	}
	mdc := md.Copy()
	return otel.GetTextMapPropagator().Extract(ctx, &mdCarrier{md: &mdc})
}

// OutgoingCall is shorthand for [Start] followed by [OutgoingContextWithTrace] for
// a client span. The returned context is ready for unary or streaming gRPC calls;
// defer [trace.Span.End] on the returned span in the same scope.
func OutgoingCall(ctx context.Context, spanName string) (context.Context, trace.Span) {
	ctx, span := Start(ctx, spanName)
	return OutgoingContextWithTrace(ctx), span
}

// OutgoingContextWithTrace copies outgoing metadata from ctx (or starts empty), injects
// the current trace, and returns a new context. Use after [Start] on the **client** side
// before unary or streaming gRPC calls.
func OutgoingContextWithTrace(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	otel.GetTextMapPropagator().Inject(ctx, &mdCarrier{md: &md})
	return metadata.NewOutgoingContext(ctx, md)
}

// ServerSpan runs [ExtractIncomingGRPC] on [metadata.FromIncomingContext] and
// [Start] for a gRPC server method. [defer trace.Span.End] in the handler.
// packagePath is the path of the handler package within the module (no module
// prefix; e.g. internal/.../handler). funcName is the exported method name
// (e.g. "RegisterController").
func ServerSpan(ctx context.Context, packagePath, funcName string) (context.Context, trace.Span) {
	md, _ := metadata.FromIncomingContext(ctx)
	ctx = ExtractIncomingGRPC(ctx, md)
	return Start(ctx, SpanName(packagePath, funcName))
}
