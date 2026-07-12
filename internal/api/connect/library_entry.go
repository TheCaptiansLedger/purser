package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// libraryEntryService is the narrow interface LibraryEntryHandler depends
// on — see personService for the DIP convention this follows.
type libraryEntryService interface {
	Create(ctx context.Context, e *domain.LibraryEntry) (*domain.LibraryEntry, error)
	Get(ctx context.Context, id string) (*domain.LibraryEntry, error)
	Update(ctx context.Context, e *domain.LibraryEntry) (*domain.LibraryEntry, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) ([]*domain.LibraryEntry, string, error)
}

// LibraryEntryHandler implements domainv1connect.LibraryEntryServiceHandler.
type LibraryEntryHandler struct {
	domainv1connect.UnimplementedLibraryEntryServiceHandler
	svc    libraryEntryService
	logger *slog.Logger
}

// NewLibraryEntryHandler constructs a LibraryEntryHandler backed by svc.
func NewLibraryEntryHandler(svc libraryEntryService, logger *slog.Logger) *LibraryEntryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &LibraryEntryHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "LibraryEntryService")}
}

// CreateLibraryEntry implements domainv1connect.LibraryEntryServiceHandler.
func (h *LibraryEntryHandler) CreateLibraryEntry(ctx context.Context, req *connect.Request[v1.CreateLibraryEntryRequest]) (*connect.Response[v1.CreateLibraryEntryResponse], error) {
	e := libraryEntryFromProto(req.Msg.GetLibraryEntry())
	created, err := h.svc.Create(ctx, e)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateLibraryEntryResponse{LibraryEntry: libraryEntryToProto(created)}), nil
}

// GetLibraryEntry implements domainv1connect.LibraryEntryServiceHandler.
func (h *LibraryEntryHandler) GetLibraryEntry(ctx context.Context, req *connect.Request[v1.GetLibraryEntryRequest]) (*connect.Response[v1.GetLibraryEntryResponse], error) {
	e, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetLibraryEntryResponse{LibraryEntry: libraryEntryToProto(e)}), nil
}

// UpdateLibraryEntry implements domainv1connect.LibraryEntryServiceHandler.
func (h *LibraryEntryHandler) UpdateLibraryEntry(ctx context.Context, req *connect.Request[v1.UpdateLibraryEntryRequest]) (*connect.Response[v1.UpdateLibraryEntryResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetLibraryEntry().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyLibraryEntryFieldMask(existing, req.Msg.GetLibraryEntry(), req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateLibraryEntryResponse{LibraryEntry: libraryEntryToProto(updated)}), nil
}

// DeleteLibraryEntry implements domainv1connect.LibraryEntryServiceHandler.
func (h *LibraryEntryHandler) DeleteLibraryEntry(ctx context.Context, req *connect.Request[v1.DeleteLibraryEntryRequest]) (*connect.Response[v1.DeleteLibraryEntryResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteLibraryEntryResponse{}), nil
}

// ListLibraryEntries implements domainv1connect.LibraryEntryServiceHandler.
func (h *LibraryEntryHandler) ListLibraryEntries(ctx context.Context, req *connect.Request[v1.ListLibraryEntriesRequest]) (*connect.Response[v1.ListLibraryEntriesResponse], error) {
	entries, next, err := h.svc.List(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbEntries := make([]*v1.LibraryEntry, 0, len(entries))
	for _, e := range entries {
		pbEntries = append(pbEntries, libraryEntryToProto(e))
	}
	return connect.NewResponse(&v1.ListLibraryEntriesResponse{LibraryEntries: pbEntries, NextPageToken: next}), nil
}
