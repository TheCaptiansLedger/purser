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

type fakeLibraryEntryService struct {
	byID      map[string]*domain.LibraryEntry
	createErr error
	getErr    error
	updateErr error
	listErr   error

	gotKind     domain.Kind
	gotParentID string
}

func newFakeLibraryEntryService() *fakeLibraryEntryService {
	return &fakeLibraryEntryService{byID: make(map[string]*domain.LibraryEntry)}
}

func (f *fakeLibraryEntryService) Create(_ context.Context, e *domain.LibraryEntry) (*domain.LibraryEntry, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[e.ID] = e
	return e, nil
}

func (f *fakeLibraryEntryService) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	e, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return e, nil
}

func (f *fakeLibraryEntryService) Update(_ context.Context, e *domain.LibraryEntry) (*domain.LibraryEntry, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[e.ID] = e
	return e, nil
}

func (f *fakeLibraryEntryService) List(_ context.Context, kind domain.Kind, parentID string, _ int, _ string) ([]*domain.LibraryEntry, string, error) {
	f.gotKind, f.gotParentID = kind, parentID
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	entries := make([]*domain.LibraryEntry, 0, len(f.byID))
	for _, e := range f.byID {
		entries = append(entries, e)
	}
	return entries, "", nil
}

func validProtoLibraryEntry(id string) *v1.LibraryEntry {
	return &v1.LibraryEntry{
		Id:          id,
		ContentType: "adult",
		Kind:        "studio",
		Name:        "Test Studio",
		MonitorMode: v1.MonitorMode_MONITOR_MODE_NONE,
	}
}

func TestLibraryEntryHandler_CreateLibraryEntry(t *testing.T) {
	t.Run("valid request returns the created entry", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)

		res, err := h.CreateLibraryEntry(context.Background(), connect.NewRequest(&v1.CreateLibraryEntryRequest{LibraryEntry: validProtoLibraryEntry("e1")}))
		if err != nil {
			t.Fatalf("CreateLibraryEntry returned error: %v", err)
		}
		if res.Msg.GetLibraryEntry().GetId() != "e1" {
			t.Fatalf("CreateLibraryEntry returned ID %q, want %q", res.Msg.GetLibraryEntry().GetId(), "e1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Name", Rule: "required", Value: ""}}}
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.CreateLibraryEntry(context.Background(), connect.NewRequest(&v1.CreateLibraryEntryRequest{LibraryEntry: validProtoLibraryEntry("e1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateLibraryEntry with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestLibraryEntryHandler_GetLibraryEntry(t *testing.T) {
	svc := newFakeLibraryEntryService()
	h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)
	svc.byID["e1"] = &domain.LibraryEntry{ID: "e1", Name: "Existing", MonitorMode: domain.MonitorModeNone}

	res, err := h.GetLibraryEntry(context.Background(), connect.NewRequest(&v1.GetLibraryEntryRequest{Id: "e1"}))
	if err != nil {
		t.Fatalf("GetLibraryEntry returned error: %v", err)
	}
	if res.Msg.GetLibraryEntry().GetName() != "Existing" {
		t.Fatalf("GetLibraryEntry returned Name %q, want %q", res.Msg.GetLibraryEntry().GetName(), "Existing")
	}

	_, err = h.GetLibraryEntry(context.Background(), connect.NewRequest(&v1.GetLibraryEntryRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetLibraryEntry on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestLibraryEntryHandler_UpdateLibraryEntry(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)
		svc.byID["e1"] = &domain.LibraryEntry{ID: "e1", Name: "Original", Overview: "Original Overview", MonitorMode: domain.MonitorModeNone}

		req := &v1.UpdateLibraryEntryRequest{
			LibraryEntry: &v1.LibraryEntry{Id: "e1", Name: "New Name", Overview: "Should be ignored"},
			UpdateMask:   &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		}
		res, err := h.UpdateLibraryEntry(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateLibraryEntry returned error: %v", err)
		}
		if res.Msg.GetLibraryEntry().GetName() != "New Name" {
			t.Fatalf("UpdateLibraryEntry applied Name %q, want %q", res.Msg.GetLibraryEntry().GetName(), "New Name")
		}
		if res.Msg.GetLibraryEntry().GetOverview() != "Original Overview" {
			t.Fatalf("UpdateLibraryEntry touched Overview: got %q", res.Msg.GetLibraryEntry().GetOverview())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.UpdateLibraryEntry(context.Background(), connect.NewRequest(&v1.UpdateLibraryEntryRequest{LibraryEntry: &v1.LibraryEntry{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateLibraryEntry on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("update failure maps through mapError", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		svc.byID["e1"] = &domain.LibraryEntry{ID: "e1", Name: "Original", MonitorMode: domain.MonitorModeNone}
		svc.updateErr = ports.ErrConflict
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.UpdateLibraryEntry(context.Background(), connect.NewRequest(&v1.UpdateLibraryEntryRequest{LibraryEntry: &v1.LibraryEntry{Id: "e1", Name: "New"}}))
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("UpdateLibraryEntry with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})
}

func TestLibraryEntryHandler_DeleteLibraryEntry(t *testing.T) {
	t.Run("valid delete succeeds and threads the cascade flag", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		deletionSvc := newFakeEntityDeletionService()
		h := apiconnect.NewLibraryEntryHandler(svc, deletionSvc, nil)

		if _, err := h.DeleteLibraryEntry(context.Background(), connect.NewRequest(&v1.DeleteLibraryEntryRequest{Id: "e1", Cascade: true})); err != nil {
			t.Fatalf("DeleteLibraryEntry returned error: %v", err)
		}
		if deletionSvc.gotID != "e1" || !deletionSvc.gotCascade {
			t.Fatalf("DeleteLibraryEntry passed (id=%q, cascade=%v), want (e1, true)", deletionSvc.gotID, deletionSvc.gotCascade)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		deletionSvc := newFakeEntityDeletionService()
		deletionSvc.deleteErr = ports.ErrDeletionBlocked
		h := apiconnect.NewLibraryEntryHandler(svc, deletionSvc, nil)

		_, err := h.DeleteLibraryEntry(context.Background(), connect.NewRequest(&v1.DeleteLibraryEntryRequest{Id: "e1"}))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("DeleteLibraryEntry blocked without cascade returned code %v, want %v", connect.CodeOf(err), connect.CodeFailedPrecondition)
		}
	})
}

func TestLibraryEntryHandler_GetLibraryEntryDeletionImpact(t *testing.T) {
	t.Run("valid request returns the impact rows", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		deletionSvc := newFakeEntityDeletionService()
		deletionSvc.impact = &domain.DeletionImpact{Impacts: []domain.DeletionImpactRow{{Kind: "group", Label: "Groups", Count: 3, Blocking: true}}}
		h := apiconnect.NewLibraryEntryHandler(svc, deletionSvc, nil)

		res, err := h.GetLibraryEntryDeletionImpact(context.Background(), connect.NewRequest(&v1.GetLibraryEntryDeletionImpactRequest{Id: "e1"}))
		if err != nil {
			t.Fatalf("GetLibraryEntryDeletionImpact returned error: %v", err)
		}
		if len(res.Msg.GetImpacts()) != 1 || res.Msg.GetImpacts()[0].GetCount() != 3 || !res.Msg.GetImpacts()[0].GetBlocking() {
			t.Fatalf("GetLibraryEntryDeletionImpact returned %v, want a single blocking row with Count 3", res.Msg.GetImpacts())
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		deletionSvc := newFakeEntityDeletionService()
		deletionSvc.impactErr = ports.ErrNotFound
		h := apiconnect.NewLibraryEntryHandler(svc, deletionSvc, nil)

		_, err := h.GetLibraryEntryDeletionImpact(context.Background(), connect.NewRequest(&v1.GetLibraryEntryDeletionImpactRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetLibraryEntryDeletionImpact on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestLibraryEntryHandler_ListLibraryEntries(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)
		svc.byID["e1"] = &domain.LibraryEntry{ID: "e1"}
		svc.byID["e2"] = &domain.LibraryEntry{ID: "e2"}

		res, err := h.ListLibraryEntries(context.Background(), connect.NewRequest(&v1.ListLibraryEntriesRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListLibraryEntries returned error: %v", err)
		}
		if len(res.Msg.GetLibraryEntries()) != 2 {
			t.Fatalf("ListLibraryEntries returned %d entries, want 2", len(res.Msg.GetLibraryEntries()))
		}
	})

	t.Run("filter fields are threaded through to the service", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.ListLibraryEntries(context.Background(), connect.NewRequest(&v1.ListLibraryEntriesRequest{
			Kind: "studio", ParentId: "network1", PageSize: 10,
		}))
		if err != nil {
			t.Fatalf("ListLibraryEntries returned error: %v", err)
		}
		if svc.gotKind != domain.KindStudio || svc.gotParentID != "network1" {
			t.Fatalf("ListLibraryEntries passed filters (%q, %q), want (%q, %q)", svc.gotKind, svc.gotParentID, domain.KindStudio, "network1")
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeLibraryEntryService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewLibraryEntryHandler(svc, newFakeEntityDeletionService(), nil)

		_, err := h.ListLibraryEntries(context.Background(), connect.NewRequest(&v1.ListLibraryEntriesRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListLibraryEntries with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
