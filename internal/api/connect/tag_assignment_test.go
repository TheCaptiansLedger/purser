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

type fakeTagAssignmentService struct {
	byKey     map[string]*domain.TagAssignment
	createErr error
	getErr    error
	deleteErr error
	listErr   error
}

func newFakeTagAssignmentService() *fakeTagAssignmentService {
	return &fakeTagAssignmentService{byKey: make(map[string]*domain.TagAssignment)}
}

func taKey(tagID string, entityType domain.EntityType, entityID string) string {
	return tagID + "|" + string(entityType) + "|" + entityID
}

func (f *fakeTagAssignmentService) Create(_ context.Context, ta *domain.TagAssignment) (*domain.TagAssignment, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byKey[taKey(ta.TagID, ta.EntityType, ta.EntityID)] = ta
	return ta, nil
}

func (f *fakeTagAssignmentService) Get(_ context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	ta, ok := f.byKey[taKey(tagID, entityType, entityID)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return ta, nil
}

func (f *fakeTagAssignmentService) Delete(_ context.Context, tagID string, entityType domain.EntityType, entityID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byKey, taKey(tagID, entityType, entityID))
	return nil
}

func (f *fakeTagAssignmentService) List(_ context.Context, _ string, _ domain.EntityType, _ string, _ int, _ string) ([]*domain.TagAssignment, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	rows := make([]*domain.TagAssignment, 0, len(f.byKey))
	for _, ta := range f.byKey {
		rows = append(rows, ta)
	}
	return rows, "", nil
}

func validProtoTagAssignment(tagID string, entityType v1.EntityType, entityID string) *v1.TagAssignment {
	return &v1.TagAssignment{TagId: tagID, EntityType: entityType, EntityId: entityID}
}

func TestTagAssignmentHandler_CreateTagAssignment(t *testing.T) {
	t.Run("valid request returns the created assignment", func(t *testing.T) {
		svc := newFakeTagAssignmentService()
		h := apiconnect.NewTagAssignmentHandler(svc, nil)

		req := &v1.CreateTagAssignmentRequest{TagAssignment: validProtoTagAssignment("t1", v1.EntityType_ENTITY_TYPE_PERSON, "p1")}
		res, err := h.CreateTagAssignment(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("CreateTagAssignment returned error: %v", err)
		}
		if res.Msg.GetTagAssignment().GetTagId() != "t1" {
			t.Fatalf("CreateTagAssignment returned TagId %q, want %q", res.Msg.GetTagAssignment().GetTagId(), "t1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeTagAssignmentService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "TagID", Rule: "required", Value: ""}}}
		h := apiconnect.NewTagAssignmentHandler(svc, nil)

		req := &v1.CreateTagAssignmentRequest{TagAssignment: validProtoTagAssignment("t1", v1.EntityType_ENTITY_TYPE_PERSON, "p1")}
		_, err := h.CreateTagAssignment(context.Background(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateTagAssignment with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestTagAssignmentHandler_GetTagAssignment(t *testing.T) {
	svc := newFakeTagAssignmentService()
	h := apiconnect.NewTagAssignmentHandler(svc, nil)
	svc.byKey[taKey("t1", domain.EntityTypePerson, "p1")] = &domain.TagAssignment{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"}

	req := &v1.GetTagAssignmentRequest{TagId: "t1", EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "p1"}
	res, err := h.GetTagAssignment(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("GetTagAssignment returned error: %v", err)
	}
	if res.Msg.GetTagAssignment().GetEntityId() != "p1" {
		t.Fatalf("GetTagAssignment returned EntityId %q, want %q", res.Msg.GetTagAssignment().GetEntityId(), "p1")
	}

	missingReq := &v1.GetTagAssignmentRequest{TagId: "t1", EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "missing"}
	_, err = h.GetTagAssignment(context.Background(), connect.NewRequest(missingReq))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetTagAssignment on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestTagAssignmentHandler_DeleteTagAssignment(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeTagAssignmentService()
		h := apiconnect.NewTagAssignmentHandler(svc, nil)
		svc.byKey[taKey("t1", domain.EntityTypePerson, "p1")] = &domain.TagAssignment{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"}

		req := &v1.DeleteTagAssignmentRequest{TagId: "t1", EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "p1"}
		if _, err := h.DeleteTagAssignment(context.Background(), connect.NewRequest(req)); err != nil {
			t.Fatalf("DeleteTagAssignment returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeTagAssignmentService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewTagAssignmentHandler(svc, nil)

		req := &v1.DeleteTagAssignmentRequest{TagId: "t1", EntityType: v1.EntityType_ENTITY_TYPE_PERSON, EntityId: "missing"}
		_, err := h.DeleteTagAssignment(context.Background(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteTagAssignment on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestTagAssignmentHandler_ListTagAssignments(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeTagAssignmentService()
		h := apiconnect.NewTagAssignmentHandler(svc, nil)
		svc.byKey[taKey("t1", domain.EntityTypePerson, "p1")] = &domain.TagAssignment{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"}
		svc.byKey[taKey("t1", domain.EntityTypePerson, "p2")] = &domain.TagAssignment{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p2"}

		res, err := h.ListTagAssignments(context.Background(), connect.NewRequest(&v1.ListTagAssignmentsRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListTagAssignments returned error: %v", err)
		}
		if len(res.Msg.GetTagAssignments()) != 2 {
			t.Fatalf("ListTagAssignments returned %d rows, want 2", len(res.Msg.GetTagAssignments()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeTagAssignmentService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewTagAssignmentHandler(svc, nil)

		_, err := h.ListTagAssignments(context.Background(), connect.NewRequest(&v1.ListTagAssignmentsRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListTagAssignments with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
