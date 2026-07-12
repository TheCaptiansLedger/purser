package apiconnect_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"

	apiconnect "purser/internal/api/connect"
)

type fakeAnyRequest struct {
	connect.AnyRequest
	procedure string
}

func (f fakeAnyRequest) Spec() connect.Spec {
	return connect.Spec{Procedure: f.procedure}
}

func TestLoggingInterceptor_WrapUnary(t *testing.T) {
	t.Run("logs a completed rpc", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		interceptor := apiconnect.NewLoggingInterceptor(logger)

		wrapped := interceptor.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			return nil, nil //nolint:nilnil // test stub: only the logging side effect is under test, the response value is irrelevant
		})

		if _, err := wrapped(context.Background(), fakeAnyRequest{procedure: "/test.Service/Method"}); err != nil {
			t.Fatalf("wrapped unary func returned error: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, `"component":"api.connect"`) {
			t.Errorf("log output missing component attribute: %s", out)
		}
		if !strings.Contains(out, `"msg":"rpc completed"`) {
			t.Errorf("log output missing completion message: %s", out)
		}
	})

	t.Run("logs a failed rpc with its code", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		interceptor := apiconnect.NewLoggingInterceptor(logger)

		wantErr := connect.NewError(connect.CodeNotFound, errors.New("not found"))
		wrapped := interceptor.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			return nil, wantErr
		})

		_, err := wrapped(context.Background(), fakeAnyRequest{procedure: "/test.Service/Method"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("wrapped unary func returned %v, want %v", err, wantErr)
		}

		out := buf.String()
		if !strings.Contains(out, `"msg":"rpc failed"`) {
			t.Errorf("log output missing failure message: %s", out)
		}
		if !strings.Contains(out, `"code":"not_found"`) {
			t.Errorf("log output missing code attribute: %s", out)
		}
	})
}

func TestLoggingInterceptor_StreamingPassThrough(t *testing.T) {
	interceptor := apiconnect.NewLoggingInterceptor(slog.Default())

	clientCalled := false
	client := interceptor.WrapStreamingClient(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		clientCalled = true
		return nil
	})
	client(context.Background(), connect.Spec{})
	if !clientCalled {
		t.Fatal("WrapStreamingClient did not pass through to the wrapped function")
	}

	handlerCalled := false
	handler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		handlerCalled = true
		return nil
	})
	_ = handler(context.Background(), nil)
	if !handlerCalled {
		t.Fatal("WrapStreamingHandler did not pass through to the wrapped function")
	}
}
