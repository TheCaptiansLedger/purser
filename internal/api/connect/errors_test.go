package apiconnect

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestMapError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	t.Run("nil is nil", func(t *testing.T) {
		if err := mapError(context.Background(), logger, nil); err != nil {
			t.Fatalf("mapError(nil) = %v, want nil", err)
		}
	})

	t.Run("ValidationError maps to CodeInvalidArgument with field details", func(t *testing.T) {
		verr := &domain.ValidationError{Errors: []domain.FieldError{{Field: "Name", Rule: "required", Value: ""}}}
		err := mapError(context.Background(), logger, verr)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("mapError(ValidationError) code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) || len(connErr.Details()) != 1 {
			t.Fatalf("mapError(ValidationError) did not attach a field-violation detail: %v", err)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		err := mapError(context.Background(), logger, ports.ErrNotFound)
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("mapError(ErrNotFound) code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("ErrConflict maps to CodeAlreadyExists", func(t *testing.T) {
		err := mapError(context.Background(), logger, ports.ErrConflict)
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("mapError(ErrConflict) code = %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})

	t.Run("ErrDeletionBlocked maps to CodeFailedPrecondition", func(t *testing.T) {
		err := mapError(context.Background(), logger, ports.ErrDeletionBlocked)
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("mapError(ErrDeletionBlocked) code = %v, want %v", connect.CodeOf(err), connect.CodeFailedPrecondition)
		}
	})

	t.Run("unmapped error becomes a generic CodeInternal and is logged", func(t *testing.T) {
		var buf bytes.Buffer
		bufLogger := slog.New(slog.NewJSONHandler(&buf, nil))

		original := errors.New("boom")
		err := mapError(context.Background(), bufLogger, original)
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("mapError(unmapped) code = %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
		if strings.Contains(err.Error(), "boom") {
			t.Fatalf("mapError(unmapped) leaked the internal error to the client: %v", err)
		}
		if !strings.Contains(buf.String(), "boom") {
			t.Fatalf("mapError(unmapped) did not log the real error server-side: %s", buf.String())
		}
	})
}
