package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// entryPersonService is the narrow interface EntryPersonHandler depends
// on — see personService for the DIP convention this follows.
type entryPersonService interface {
	Create(ctx context.Context, ep *domain.EntryPerson) (*domain.EntryPerson, error)
	Get(ctx context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error)
	Update(ctx context.Context, ep *domain.EntryPerson) (*domain.EntryPerson, error)
	Delete(ctx context.Context, libraryEntryID, personID, role string) error
	List(ctx context.Context, libraryEntryID, personID string, pageSize int, pageToken string) ([]*domain.EntryPerson, string, error)
}

// EntryPersonHandler implements domainv1connect.EntryPersonServiceHandler.
type EntryPersonHandler struct {
	domainv1connect.UnimplementedEntryPersonServiceHandler
	svc    entryPersonService
	logger *slog.Logger
}

// NewEntryPersonHandler constructs an EntryPersonHandler backed by svc.
func NewEntryPersonHandler(svc entryPersonService, logger *slog.Logger) *EntryPersonHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EntryPersonHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "EntryPersonService")}
}

// CreateEntryPerson implements domainv1connect.EntryPersonServiceHandler.
func (h *EntryPersonHandler) CreateEntryPerson(ctx context.Context, req *connect.Request[v1.CreateEntryPersonRequest]) (*connect.Response[v1.CreateEntryPersonResponse], error) {
	ep := entryPersonFromProto(req.Msg.GetEntryPerson())
	created, err := h.svc.Create(ctx, ep)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreateEntryPersonResponse{EntryPerson: entryPersonToProto(created)}), nil
}

// GetEntryPerson implements domainv1connect.EntryPersonServiceHandler.
func (h *EntryPersonHandler) GetEntryPerson(ctx context.Context, req *connect.Request[v1.GetEntryPersonRequest]) (*connect.Response[v1.GetEntryPersonResponse], error) {
	ep, err := h.svc.Get(ctx, req.Msg.GetLibraryEntryId(), req.Msg.GetPersonId(), req.Msg.GetRole())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetEntryPersonResponse{EntryPerson: entryPersonToProto(ep)}), nil
}

// UpdateEntryPerson implements domainv1connect.EntryPersonServiceHandler.
func (h *EntryPersonHandler) UpdateEntryPerson(ctx context.Context, req *connect.Request[v1.UpdateEntryPersonRequest]) (*connect.Response[v1.UpdateEntryPersonResponse], error) {
	incoming := req.Msg.GetEntryPerson()
	existing, err := h.svc.Get(ctx, incoming.GetLibraryEntryId(), incoming.GetPersonId(), incoming.GetRole())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyEntryPersonFieldMask(existing, incoming, req.Msg.GetUpdateMask())

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdateEntryPersonResponse{EntryPerson: entryPersonToProto(updated)}), nil
}

// DeleteEntryPerson implements domainv1connect.EntryPersonServiceHandler.
func (h *EntryPersonHandler) DeleteEntryPerson(ctx context.Context, req *connect.Request[v1.DeleteEntryPersonRequest]) (*connect.Response[v1.DeleteEntryPersonResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetLibraryEntryId(), req.Msg.GetPersonId(), req.Msg.GetRole()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeleteEntryPersonResponse{}), nil
}

// ListEntryPeople implements domainv1connect.EntryPersonServiceHandler.
func (h *EntryPersonHandler) ListEntryPeople(ctx context.Context, req *connect.Request[v1.ListEntryPeopleRequest]) (*connect.Response[v1.ListEntryPeopleResponse], error) {
	rows, next, err := h.svc.List(ctx, req.Msg.GetLibraryEntryId(), req.Msg.GetPersonId(), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbRows := make([]*v1.EntryPerson, 0, len(rows))
	for _, ep := range rows {
		pbRows = append(pbRows, entryPersonToProto(ep))
	}
	return connect.NewResponse(&v1.ListEntryPeopleResponse{EntryPeople: pbRows, NextPageToken: next}), nil
}
