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

type fakeItemService struct {
	byID      map[string]*domain.Item
	createErr error
	getErr    error
	updateErr error
	listErr   error

	gotLibraryEntryID string
	gotContentType    string
	gotGroupID        string
	gotStatus         domain.ItemStatus
}

func newFakeItemService() *fakeItemService {
	return &fakeItemService{byID: make(map[string]*domain.Item)}
}

func (f *fakeItemService) Create(_ context.Context, i *domain.Item) (*domain.Item, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[i.ID] = i
	return i, nil
}

func (f *fakeItemService) Get(_ context.Context, id string) (*domain.Item, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	i, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return i, nil
}

func (f *fakeItemService) Update(_ context.Context, i *domain.Item) (*domain.Item, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[i.ID] = i
	return i, nil
}

func (f *fakeItemService) List(_ context.Context, libraryEntryID, contentType, groupID string, status domain.ItemStatus, _ int, _ string) ([]*domain.Item, string, error) {
	f.gotLibraryEntryID, f.gotContentType, f.gotGroupID, f.gotStatus = libraryEntryID, contentType, groupID, status
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	items := make([]*domain.Item, 0, len(f.byID))
	for _, i := range f.byID {
		items = append(items, i)
	}
	return items, "", nil
}

func validProtoItem(id string) *v1.Item {
	return &v1.Item{Id: id, ContentType: "adult", LibraryEntryId: "entry1", Title: "Test Item", Status: v1.ItemStatus_ITEM_STATUS_WANTED}
}

func TestItemHandler_CreateItem(t *testing.T) {
	t.Run("valid request returns the created item", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)

		res, err := h.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Item: validProtoItem("i1")}))
		if err != nil {
			t.Fatalf("CreateItem returned error: %v", err)
		}
		if res.Msg.GetItem().GetId() != "i1" {
			t.Fatalf("CreateItem returned ID %q, want %q", res.Msg.GetItem().GetId(), "i1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeItemService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Title", Rule: "required", Value: ""}}}
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)

		_, err := h.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Item: validProtoItem("i1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateItem with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestItemHandler_GetItem(t *testing.T) {
	svc := newFakeItemService()
	h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)
	svc.byID["i1"] = &domain.Item{ID: "i1", Title: "Existing", Status: domain.ItemStatusWanted}

	res, err := h.GetItem(context.Background(), connect.NewRequest(&v1.GetItemRequest{Id: "i1"}))
	if err != nil {
		t.Fatalf("GetItem returned error: %v", err)
	}
	if res.Msg.GetItem().GetTitle() != "Existing" {
		t.Fatalf("GetItem returned Title %q, want %q", res.Msg.GetItem().GetTitle(), "Existing")
	}

	_, err = h.GetItem(context.Background(), connect.NewRequest(&v1.GetItemRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetItem on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestItemHandler_UpdateItem(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)
		svc.byID["i1"] = &domain.Item{ID: "i1", Title: "Original", Overview: "Original Overview", Status: domain.ItemStatusWanted}

		req := &v1.UpdateItemRequest{
			Item:       &v1.Item{Id: "i1", Title: "New Title", Overview: "Should be ignored"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		}
		res, err := h.UpdateItem(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateItem returned error: %v", err)
		}
		if res.Msg.GetItem().GetTitle() != "New Title" {
			t.Fatalf("UpdateItem applied Title %q, want %q", res.Msg.GetItem().GetTitle(), "New Title")
		}
		if res.Msg.GetItem().GetOverview() != "Original Overview" {
			t.Fatalf("UpdateItem touched Overview: got %q", res.Msg.GetItem().GetOverview())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)

		_, err := h.UpdateItem(context.Background(), connect.NewRequest(&v1.UpdateItemRequest{Item: &v1.Item{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateItem on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("update failure maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		svc.byID["i1"] = &domain.Item{ID: "i1", Title: "Original", Status: domain.ItemStatusWanted}
		svc.updateErr = ports.ErrConflict
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)

		_, err := h.UpdateItem(context.Background(), connect.NewRequest(&v1.UpdateItemRequest{Item: &v1.Item{Id: "i1", Title: "New"}}))
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("UpdateItem with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})
}

func TestItemHandler_DeleteItem(t *testing.T) {
	t.Run("valid delete succeeds and threads the cascade flag", func(t *testing.T) {
		svc := newFakeItemService()
		deletionSvc := newFakeBulkDeletionService()
		h := apiconnect.NewItemHandler(svc, deletionSvc, nil)

		if _, err := h.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "i1", Cascade: true})); err != nil {
			t.Fatalf("DeleteItem returned error: %v", err)
		}
		if deletionSvc.gotID != "i1" || !deletionSvc.gotCascade {
			t.Fatalf("DeleteItem passed (id=%q, cascade=%v), want (i1, true)", deletionSvc.gotID, deletionSvc.gotCascade)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		deletionSvc := newFakeBulkDeletionService()
		deletionSvc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewItemHandler(svc, deletionSvc, nil)

		_, err := h.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteItem on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestItemHandler_GetItemDeletionImpact(t *testing.T) {
	t.Run("valid request returns the impact rows", func(t *testing.T) {
		svc := newFakeItemService()
		deletionSvc := newFakeBulkDeletionService()
		deletionSvc.impact = &domain.DeletionImpact{Impacts: []domain.DeletionImpactRow{{Kind: "item_person", Label: "Credits", Count: 2}}}
		h := apiconnect.NewItemHandler(svc, deletionSvc, nil)

		res, err := h.GetItemDeletionImpact(context.Background(), connect.NewRequest(&v1.GetItemDeletionImpactRequest{Id: "i1"}))
		if err != nil {
			t.Fatalf("GetItemDeletionImpact returned error: %v", err)
		}
		if len(res.Msg.GetImpacts()) != 1 || res.Msg.GetImpacts()[0].GetCount() != 2 {
			t.Fatalf("GetItemDeletionImpact returned %v, want a single row with Count 2", res.Msg.GetImpacts())
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		deletionSvc := newFakeBulkDeletionService()
		deletionSvc.impactErr = ports.ErrNotFound
		h := apiconnect.NewItemHandler(svc, deletionSvc, nil)

		_, err := h.GetItemDeletionImpact(context.Background(), connect.NewRequest(&v1.GetItemDeletionImpactRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetItemDeletionImpact on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestItemHandler_ListItems(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)
		svc.byID["i1"] = &domain.Item{ID: "i1"}
		svc.byID["i2"] = &domain.Item{ID: "i2"}

		res, err := h.ListItems(context.Background(), connect.NewRequest(&v1.ListItemsRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListItems returned error: %v", err)
		}
		if len(res.Msg.GetItems()) != 2 {
			t.Fatalf("ListItems returned %d items, want 2", len(res.Msg.GetItems()))
		}
	})

	t.Run("filter fields are threaded through to the service", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)

		_, err := h.ListItems(context.Background(), connect.NewRequest(&v1.ListItemsRequest{
			LibraryEntryId: "studio1", ContentType: "adult", GroupId: "group1", Status: v1.ItemStatus_ITEM_STATUS_WANTED, PageSize: 10,
		}))
		if err != nil {
			t.Fatalf("ListItems returned error: %v", err)
		}
		if svc.gotLibraryEntryID != "studio1" || svc.gotContentType != "adult" || svc.gotGroupID != "group1" || svc.gotStatus != domain.ItemStatusWanted {
			t.Fatalf("ListItems passed filters (%q, %q, %q, %q), want (%q, %q, %q, %q)",
				svc.gotLibraryEntryID, svc.gotContentType, svc.gotGroupID, svc.gotStatus, "studio1", "adult", "group1", domain.ItemStatusWanted)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewItemHandler(svc, newFakeBulkDeletionService(), nil)

		_, err := h.ListItems(context.Background(), connect.NewRequest(&v1.ListItemsRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListItems with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestItemHandler_BulkDeleteItems(t *testing.T) {
	t.Run("valid request threads ids and cascade through to the service", func(t *testing.T) {
		svc := newFakeItemService()
		deletionSvc := newFakeBulkDeletionService()
		h := apiconnect.NewItemHandler(svc, deletionSvc, nil)

		req := &v1.BulkDeleteItemsRequest{Ids: []string{"i1", "i2"}, Cascade: true}
		if _, err := h.BulkDeleteItems(context.Background(), connect.NewRequest(req)); err != nil {
			t.Fatalf("BulkDeleteItems returned error: %v", err)
		}
		if len(deletionSvc.gotBatchIDs) != 2 || !deletionSvc.gotBatchCascade {
			t.Fatalf("BulkDeleteItems passed (ids=%v, cascade=%v), want ([i1 i2], true)", deletionSvc.gotBatchIDs, deletionSvc.gotBatchCascade)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		deletionSvc := newFakeBulkDeletionService()
		deletionSvc.deleteBatchErr = ports.ErrNotFound
		h := apiconnect.NewItemHandler(svc, deletionSvc, nil)

		_, err := h.BulkDeleteItems(context.Background(), connect.NewRequest(&v1.BulkDeleteItemsRequest{Ids: []string{"missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("BulkDeleteItems on a missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}
