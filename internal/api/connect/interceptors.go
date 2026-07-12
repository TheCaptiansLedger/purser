package apiconnect

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/trace"
)

// spanAttrs returns trace_id/span_id log attributes for ctx's active span,
// or nil if there isn't one — per docs/adr/0008-structured-logging.md.
func spanAttrs(ctx context.Context) []any {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []any{"trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String()}
}

// NewLoggingInterceptor returns a connect.Interceptor that logs every
// unary RPC (procedure, status code, duration) at component=api.connect,
// per docs/adr/0008-structured-logging.md. Wire it into the server's
// interceptor chain in cmd/purser alongside connectrpc.com/otelconnect's
// tracing/metrics interceptor — this one only logs, per
// docs/adr/0007-telemetry.md's separation of concerns.
func NewLoggingInterceptor(logger *slog.Logger) connect.Interceptor {
	return &loggingInterceptor{logger: logger.With("component", "api.connect")}
}

type loggingInterceptor struct {
	logger *slog.Logger
}

func (i *loggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		res, err := next(ctx, req)

		attrs := append([]any{
			"procedure", req.Spec().Procedure,
			"duration_ms", time.Since(start).Milliseconds(),
		}, spanAttrs(ctx)...)

		if err != nil {
			attrs = append(attrs, "code", connect.CodeOf(err).String(), "error", err)
			i.logger.ErrorContext(ctx, "rpc failed", attrs...)
			return res, err
		}
		i.logger.InfoContext(ctx, "rpc completed", attrs...)
		return res, err
	}
}

func (i *loggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *loggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
