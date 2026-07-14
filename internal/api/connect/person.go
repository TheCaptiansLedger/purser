package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"
	"time"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// personService is the narrow interface PersonHandler depends on —
// satisfied by *service.PersonService, and fakeable in tests without a
// repository. Applying ADR 0002's DIP one layer above the port boundary:
// the handler depends on this interface, never the concrete service type.
type personService interface {
	Create(ctx context.Context, p *domain.Person) (*domain.Person, error)
	Get(ctx context.Context, id string) (*domain.Person, error)
	Update(ctx context.Context, p *domain.Person) (*domain.Person, error)
	List(ctx context.Context, pageSize int, pageToken string) ([]*domain.Person, string, error)
}

// PersonHandler implements domainv1connect.PersonServiceHandler — the
// driving adapter translating between the wire and personService. It
// holds no business logic and never enriches a caller's input with
// metadata; see docs/adr/0011-api-design.md.
type PersonHandler struct {
	domainv1connect.UnimplementedPersonServiceHandler
	svc         personService
	deletionSvc entityDeletionService
	logger      *slog.Logger
}

// NewPersonHandler constructs a PersonHandler backed by svc and deletionSvc.
func NewPersonHandler(svc personService, deletionSvc entityDeletionService, logger *slog.Logger) *PersonHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PersonHandler{svc: svc, deletionSvc: deletionSvc, logger: logger.With("component", "api.connect", "service", "PersonService")}
}

// CreatePerson implements domainv1connect.PersonServiceHandler. The caller
// supplies a fully-formed Person; AddedAt/UpdatedAt are server-assigned.
func (h *PersonHandler) CreatePerson(ctx context.Context, req *connect.Request[v1.CreatePersonRequest]) (*connect.Response[v1.CreatePersonResponse], error) {
	p := personFromProto(req.Msg.GetPerson())
	now := time.Now()
	p.AddedAt = now
	p.UpdatedAt = now

	created, err := h.svc.Create(ctx, p)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.CreatePersonResponse{Person: personToProto(created)}), nil
}

// GetPerson implements domainv1connect.PersonServiceHandler.
func (h *PersonHandler) GetPerson(ctx context.Context, req *connect.Request[v1.GetPersonRequest]) (*connect.Response[v1.GetPersonResponse], error) {
	p, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetPersonResponse{Person: personToProto(p)}), nil
}

// UpdatePerson implements domainv1connect.PersonServiceHandler. The
// field-mask merge (a wire-transport concept) happens here, not in
// internal/service — see docs/adr/0011-api-design.md.
func (h *PersonHandler) UpdatePerson(ctx context.Context, req *connect.Request[v1.UpdatePersonRequest]) (*connect.Response[v1.UpdatePersonResponse], error) {
	existing, err := h.svc.Get(ctx, req.Msg.GetPerson().GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	merged := applyPersonFieldMask(existing, req.Msg.GetPerson(), req.Msg.GetUpdateMask())
	merged.AddedAt = existing.AddedAt
	merged.UpdatedAt = time.Now()

	updated, err := h.svc.Update(ctx, merged)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.UpdatePersonResponse{Person: personToProto(updated)}), nil
}

// DeletePerson implements domainv1connect.PersonServiceHandler.
func (h *PersonHandler) DeletePerson(ctx context.Context, req *connect.Request[v1.DeletePersonRequest]) (*connect.Response[v1.DeletePersonResponse], error) {
	if err := h.deletionSvc.Delete(ctx, req.Msg.GetId(), req.Msg.GetCascade()); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.DeletePersonResponse{}), nil
}

// GetPersonDeletionImpact implements domainv1connect.PersonServiceHandler.
func (h *PersonHandler) GetPersonDeletionImpact(ctx context.Context, req *connect.Request[v1.GetPersonDeletionImpactRequest]) (*connect.Response[v1.GetPersonDeletionImpactResponse], error) {
	impact, err := h.deletionSvc.GetDeletionImpact(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&v1.GetPersonDeletionImpactResponse{Impacts: deletionImpactRowsToProto(impact.Impacts)}), nil
}

// ListPeople implements domainv1connect.PersonServiceHandler.
func (h *PersonHandler) ListPeople(ctx context.Context, req *connect.Request[v1.ListPeopleRequest]) (*connect.Response[v1.ListPeopleResponse], error) {
	people, next, err := h.svc.List(ctx, int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbPeople := make([]*v1.Person, 0, len(people))
	for _, p := range people {
		pbPeople = append(pbPeople, personToProto(p))
	}
	return connect.NewResponse(&v1.ListPeopleResponse{People: pbPeople, NextPageToken: next}), nil
}
