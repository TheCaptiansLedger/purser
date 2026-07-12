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
	deleteErr error
	listErr   error
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

func (f *fakeItemService) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeItemService) List(_ context.Context, _ int, _ string) ([]*domain.Item, string, error) {
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
		h := apiconnect.NewItemHandler(svc, nil)

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
		h := apiconnect.NewItemHandler(svc, nil)

		_, err := h.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Item: validProtoItem("i1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateItem with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestItemHandler_GetItem(t *testing.T) {
	svc := newFakeItemService()
	h := apiconnect.NewItemHandler(svc, nil)
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
		h := apiconnect.NewItemHandler(svc, nil)
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
		h := apiconnect.NewItemHandler(svc, nil)

		_, err := h.UpdateItem(context.Background(), connect.NewRequest(&v1.UpdateItemRequest{Item: &v1.Item{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateItem on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("update failure maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		svc.byID["i1"] = &domain.Item{ID: "i1", Title: "Original", Status: domain.ItemStatusWanted}
		svc.updateErr = ports.ErrConflict
		h := apiconnect.NewItemHandler(svc, nil)

		_, err := h.UpdateItem(context.Background(), connect.NewRequest(&v1.UpdateItemRequest{Item: &v1.Item{Id: "i1", Title: "New"}}))
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("UpdateItem with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})
}

func TestItemHandler_DeleteItem(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, nil)
		svc.byID["i1"] = &domain.Item{ID: "i1"}

		if _, err := h.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "i1"})); err != nil {
			t.Fatalf("DeleteItem returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewItemHandler(svc, nil)

		_, err := h.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteItem on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestItemHandler_ListItems(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeItemService()
		h := apiconnect.NewItemHandler(svc, nil)
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

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewItemHandler(svc, nil)

		_, err := h.ListItems(context.Background(), connect.NewRequest(&v1.ListItemsRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListItems with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
