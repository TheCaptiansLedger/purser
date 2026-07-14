package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeGroupService struct {
	byID      map[string]*domain.Group
	createErr error
	getErr    error
	updateErr error
	listErr   error
}

func newFakeGroupService() *fakeGroupService {
	return &fakeGroupService{byID: make(map[string]*domain.Group)}
}

func (f *fakeGroupService) Create(_ context.Context, g *domain.Group) (*domain.Group, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[g.ID] = g
	return g, nil
}

func (f *fakeGroupService) Get(_ context.Context, id string) (*domain.Group, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	g, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return g, nil
}

func (f *fakeGroupService) Update(_ context.Context, g *domain.Group) (*domain.Group, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[g.ID] = g
	return g, nil
}

func (f *fakeGroupService) List(_ context.Context, _ string, _ int, _ string) ([]*domain.Group, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	groups := make([]*domain.Group, 0, len(f.byID))
	for _, g := range f.byID {
		groups = append(groups, g)
	}
	return groups, "", nil
}

func validProtoGroup(id string) *v1.Group {
	return &v1.Group{Id: id, LibraryEntryId: "entry1", Title: "Test Group", MonitorMode: v1.MonitorMode_MONITOR_MODE_NONE}
}

func TestGroupHandler_CreateGroup(t *testing.T) {
	t.Run("valid request returns the created group", func(t *testing.T) {
		svc := newFakeGroupService()
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)

		res, err := h.CreateGroup(context.Background(), connect.NewRequest(&v1.CreateGroupRequest{Group: validProtoGroup("g1")}))
		if err != nil {
			t.Fatalf("CreateGroup returned error: %v", err)
		}
		if res.Msg.GetGroup().GetId() != "g1" {
			t.Fatalf("CreateGroup returned ID %q, want %q", res.Msg.GetGroup().GetId(), "g1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeGroupService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Title", Rule: "required", Value: ""}}}
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.CreateGroup(context.Background(), connect.NewRequest(&v1.CreateGroupRequest{Group: validProtoGroup("g1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateGroup with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestGroupHandler_GetGroup(t *testing.T) {
	svc := newFakeGroupService()
	h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)
	svc.byID["g1"] = &domain.Group{ID: "g1", Title: "Existing", MonitorMode: domain.MonitorModeNone}

	res, err := h.GetGroup(context.Background(), connect.NewRequest(&v1.GetGroupRequest{Id: "g1"}))
	if err != nil {
		t.Fatalf("GetGroup returned error: %v", err)
	}
	if res.Msg.GetGroup().GetTitle() != "Existing" {
		t.Fatalf("GetGroup returned Title %q, want %q", res.Msg.GetGroup().GetTitle(), "Existing")
	}

	_, err = h.GetGroup(context.Background(), connect.NewRequest(&v1.GetGroupRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetGroup on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestGroupHandler_UpdateGroup(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeGroupService()
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)
		svc.byID["g1"] = &domain.Group{ID: "g1", Title: "Original", Overview: "Original Overview", MonitorMode: domain.MonitorModeNone}

		req := &v1.UpdateGroupRequest{
			Group:      &v1.Group{Id: "g1", Title: "New Title", Overview: "Should be ignored"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		}
		res, err := h.UpdateGroup(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateGroup returned error: %v", err)
		}
		if res.Msg.GetGroup().GetTitle() != "New Title" {
			t.Fatalf("UpdateGroup applied Title %q, want %q", res.Msg.GetGroup().GetTitle(), "New Title")
		}
		if res.Msg.GetGroup().GetOverview() != "Original Overview" {
			t.Fatalf("UpdateGroup touched Overview: got %q", res.Msg.GetGroup().GetOverview())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeGroupService()
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.UpdateGroup(context.Background(), connect.NewRequest(&v1.UpdateGroupRequest{Group: &v1.Group{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateGroup on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("update failure maps through mapError", func(t *testing.T) {
		svc := newFakeGroupService()
		svc.byID["g1"] = &domain.Group{ID: "g1", Title: "Original", MonitorMode: domain.MonitorModeNone}
		svc.updateErr = ports.ErrConflict
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.UpdateGroup(context.Background(), connect.NewRequest(&v1.UpdateGroupRequest{Group: &v1.Group{Id: "g1", Title: "New"}}))
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("UpdateGroup with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})
}

func TestGroupHandler_DeleteGroup(t *testing.T) {
	t.Run("valid delete succeeds and threads the cascade flag", func(t *testing.T) {
		svc := newFakeGroupService()
		deletionSvc := newFakeEntityDeletionService()
		h := apiconnect.NewGroupHandler(svc, deletionSvc, nil)

		if _, err := h.DeleteGroup(context.Background(), connect.NewRequest(&v1.DeleteGroupRequest{Id: "g1", Cascade: true})); err != nil {
			t.Fatalf("DeleteGroup returned error: %v", err)
		}
		if deletionSvc.gotID != "g1" || !deletionSvc.gotCascade {
			t.Fatalf("DeleteGroup passed (id=%q, cascade=%v), want (g1, true)", deletionSvc.gotID, deletionSvc.gotCascade)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeGroupService()
		deletionSvc := newFakeEntityDeletionService()
		deletionSvc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewGroupHandler(svc, deletionSvc, nil)

		_, err := h.DeleteGroup(context.Background(), connect.NewRequest(&v1.DeleteGroupRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteGroup on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestGroupHandler_GetGroupDeletionImpact(t *testing.T) {
	t.Run("valid request returns the impact rows", func(t *testing.T) {
		svc := newFakeGroupService()
		deletionSvc := newFakeEntityDeletionService()
		deletionSvc.impact = &domain.DeletionImpact{Impacts: []domain.DeletionImpactRow{{Kind: "item", Label: "Items (will be detached, not deleted)", Count: 5}}}
		h := apiconnect.NewGroupHandler(svc, deletionSvc, nil)

		res, err := h.GetGroupDeletionImpact(context.Background(), connect.NewRequest(&v1.GetGroupDeletionImpactRequest{Id: "g1"}))
		if err != nil {
			t.Fatalf("GetGroupDeletionImpact returned error: %v", err)
		}
		if len(res.Msg.GetImpacts()) != 1 || res.Msg.GetImpacts()[0].GetCount() != 5 {
			t.Fatalf("GetGroupDeletionImpact returned %v, want a single row with Count 5", res.Msg.GetImpacts())
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeGroupService()
		deletionSvc := newFakeEntityDeletionService()
		deletionSvc.impactErr = ports.ErrNotFound
		h := apiconnect.NewGroupHandler(svc, deletionSvc, nil)

		_, err := h.GetGroupDeletionImpact(context.Background(), connect.NewRequest(&v1.GetGroupDeletionImpactRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetGroupDeletionImpact on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestGroupHandler_ListGroups(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeGroupService()
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)
		svc.byID["g1"] = &domain.Group{ID: "g1"}
		svc.byID["g2"] = &domain.Group{ID: "g2"}

		res, err := h.ListGroups(context.Background(), connect.NewRequest(&v1.ListGroupsRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListGroups returned error: %v", err)
		}
		if len(res.Msg.GetGroups()) != 2 {
			t.Fatalf("ListGroups returned %d groups, want 2", len(res.Msg.GetGroups()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeGroupService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewGroupHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.ListGroups(context.Background(), connect.NewRequest(&v1.ListGroupsRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListGroups with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
