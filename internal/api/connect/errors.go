// Package apiconnect is the Connect driving adapter — generated-interface
// implementations that translate between the wire (proto) and
// internal/service. No business logic lives here; see
// docs/adr/0011-api-design.md and docs/adr/0001-hexagonal-architecture.md.
//
// Named apiconnect rather than connect to avoid every call site needing an
// import alias against connectrpc.com/connect.
package apiconnect

import (
	"context"
	"errors"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/jobqueue"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

// mapError translates a service/port error into the connect.Error a
// handler returns to the caller. Every RPC error path goes through this
// helper rather than constructing connect.NewError calls ad hoc per
// handler, per docs/adr/0011-api-design.md's error-mapping convention.
func mapError(ctx context.Context, logger *slog.Logger, err error) error {
	if err == nil {
		return nil
	}

	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		connErr := connect.NewError(connect.CodeInvalidArgument, err)
		violations := make([]*errdetails.BadRequest_FieldViolation, 0, len(verr.Errors))
		for _, fe := range verr.Errors {
			violations = append(violations, &errdetails.BadRequest_FieldViolation{
				Field:       fe.Field,
				Description: fe.String(),
			})
		}
		if detail, dErr := connect.NewErrorDetail(&errdetails.BadRequest{FieldViolations: violations}); dErr == nil {
			connErr.AddDetail(detail)
		}
		return connErr
	}

	switch {
	case errors.Is(err, ports.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, ports.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, ports.ErrDeletionBlocked):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, ports.ErrDestinationExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, ports.ErrDestinationOutsideRoot):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, jobqueue.ErrUnknownKind):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		logUnmapped(ctx, logger, err)
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}

// logUnmapped records the real error server-side, correlated to the active
// span if one exists — the client only ever sees a generic CodeInternal
// message for anything that isn't a recognized validation/not-found/
// conflict error, per docs/adr/0008-structured-logging.md.
func logUnmapped(ctx context.Context, logger *slog.Logger, err error) {
	attrs := spanAttrs(ctx)
	attrs = append(attrs, "error", err)
	logger.ErrorContext(ctx, "unmapped service error", attrs...)
}
