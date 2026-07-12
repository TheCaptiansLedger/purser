package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeExternalIDService struct {
	byKey     map[string]*domain.ExternalID
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeExternalIDService() *fakeExternalIDService {
	return &fakeExternalIDService{byKey: make(map[string]*domain.ExternalID)}
}

func eidKey(entityType domain.EntityType, entityID, source string) string {
	return string(entityType) + "|" + entityID + "|" + source
}

func (f *fakeExternalIDService) Create(_ context.Context, e *domain.ExternalID) (*domain.ExternalID, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byKey[eidKey(e.EntityType, e.EntityID, string(e.Source))] = e
	return e, nil
}

func (f *fakeExternalIDService) Get(_ context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	e, ok := f.byKey[eidKey(entityType, entityID, source)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return e, nil
}

func (f *fakeExternalIDService) Update(_ context.Context, e *domain.ExternalID) (*domain.ExternalID, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byKey[eidKey(e.EntityType, e.EntityID, string(e.Source))] = e
	return e, nil
}

func (f *fakeExternalIDService) Delete(_ context.Context, entityType domain.EntityType, entityID, source string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byKey, eidKey(entityType, entityID, source))
	return nil
}

func (f *fakeExternalIDService) List(_ context.Context, _ domain.EntityType, _ string, _ int, _ string) ([]*domain.ExternalID, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	rows := make([]*domain.ExternalID, 0, len(f.byKey))
	for _, e := range f.byKey {
		rows = append(rows, e)
	}
	return rows, "", nil
}

func validProtoExternalID(entityID, source string) *v1.ExternalID {
	return &v1.ExternalID{EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: entityID, Source: source, Value: "v1"}
}

func TestExternalIDHandler_CreateExternalID(t *testing.T) {
	t.Run("valid request returns the created external id", func(t *testing.T) {
		svc := newFakeExternalIDService()
		h := apiconnect.NewExternalIDHandler(svc, nil)

		res, err := h.CreateExternalID(context.Background(), connect.NewRequest(&v1.CreateExternalIDRequest{ExternalId: validProtoExternalID("p1", "stashdb")}))
		if err != nil {
			t.Fatalf("CreateExternalID returned error: %v", err)
		}
		if res.Msg.GetExternalId().GetEntityId() != "p1" {
			t.Fatalf("CreateExternalID returned EntityId %q, want %q", res.Msg.GetExternalId().GetEntityId(), "p1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeExternalIDService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Value", Rule: "required", Value: ""}}}
		h := apiconnect.NewExternalIDHandler(svc, nil)

		_, err := h.CreateExternalID(context.Background(), connect.NewRequest(&v1.CreateExternalIDRequest{ExternalId: validProtoExternalID("p1", "stashdb")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateExternalID with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestExternalIDHandler_GetExternalID(t *testing.T) {
	svc := newFakeExternalIDService()
	h := apiconnect.NewExternalIDHandler(svc, nil)
	svc.byKey[eidKey(domain.EntityTypePerson, "p1", "stashdb")] = &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: "stashdb", Value: "Existing"}

	res, err := h.GetExternalID(context.Background(), connect.NewRequest(&v1.GetExternalIDRequest{EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "p1", Source: "stashdb"}))
	if err != nil {
		t.Fatalf("GetExternalID returned error: %v", err)
	}
	if res.Msg.GetExternalId().GetValue() != "Existing" {
		t.Fatalf("GetExternalID returned Value %q, want %q", res.Msg.GetExternalId().GetValue(), "Existing")
	}

	_, err = h.GetExternalID(context.Background(), connect.NewRequest(&v1.GetExternalIDRequest{EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "missing", Source: "stashdb"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetExternalID on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestExternalIDHandler_UpdateExternalID(t *testing.T) {
	t.Run("only value is applied, key fields are pinned", func(t *testing.T) {
		svc := newFakeExternalIDService()
		h := apiconnect.NewExternalIDHandler(svc, nil)
		svc.byKey[eidKey(domain.EntityTypePerson, "p1", "stashdb")] = &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: "stashdb", Value: "Original"}

		req := &v1.UpdateExternalIDRequest{ExternalId: &v1.ExternalID{EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "p1", Source: "stashdb", Value: "New"}}
		res, err := h.UpdateExternalID(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateExternalID returned error: %v", err)
		}
		if res.Msg.GetExternalId().GetValue() != "New" {
			t.Fatalf("UpdateExternalID applied Value %q, want %q", res.Msg.GetExternalId().GetValue(), "New")
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeExternalIDService()
		h := apiconnect.NewExternalIDHandler(svc, nil)

		_, err := h.UpdateExternalID(context.Background(), connect.NewRequest(&v1.UpdateExternalIDRequest{ExternalId: validProtoExternalID("missing", "stashdb")}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateExternalID on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestExternalIDHandler_DeleteExternalID(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeExternalIDService()
		h := apiconnect.NewExternalIDHandler(svc, nil)
		svc.byKey[eidKey(domain.EntityTypePerson, "p1", "stashdb")] = &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: "stashdb"}

		req := &v1.DeleteExternalIDRequest{EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "p1", Source: "stashdb"}
		if _, err := h.DeleteExternalID(context.Background(), connect.NewRequest(req)); err != nil {
			t.Fatalf("DeleteExternalID returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeExternalIDService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewExternalIDHandler(svc, nil)

		_, err := h.DeleteExternalID(context.Background(), connect.NewRequest(&v1.DeleteExternalIDRequest{EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "missing", Source: "stashdb"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteExternalID on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestExternalIDHandler_ListExternalIDs(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeExternalIDService()
		h := apiconnect.NewExternalIDHandler(svc, nil)
		svc.byKey[eidKey(domain.EntityTypePerson, "p1", "stashdb")] = &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: "stashdb"}
		svc.byKey[eidKey(domain.EntityTypePerson, "p1", "tpdb")] = &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: "tpdb"}

		res, err := h.ListExternalIDs(context.Background(), connect.NewRequest(&v1.ListExternalIDsRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListExternalIDs returned error: %v", err)
		}
		if len(res.Msg.GetExternalIds()) != 2 {
			t.Fatalf("ListExternalIDs returned %d rows, want 2", len(res.Msg.GetExternalIds()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeExternalIDService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewExternalIDHandler(svc, nil)

		_, err := h.ListExternalIDs(context.Background(), connect.NewRequest(&v1.ListExternalIDsRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListExternalIDs with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
