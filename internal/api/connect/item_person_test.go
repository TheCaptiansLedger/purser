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

type fakeItemPersonService struct {
	byKey     map[string]*domain.ItemPerson
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeItemPersonService() *fakeItemPersonService {
	return &fakeItemPersonService{byKey: make(map[string]*domain.ItemPerson)}
}

func ipKey(itemID, personID, role string) string {
	return itemID + "|" + personID + "|" + role
}

func (f *fakeItemPersonService) Create(_ context.Context, ip *domain.ItemPerson) (*domain.ItemPerson, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byKey[ipKey(ip.ItemID, ip.PersonID, ip.Role)] = ip
	return ip, nil
}

func (f *fakeItemPersonService) Get(_ context.Context, itemID, personID, role string) (*domain.ItemPerson, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	ip, ok := f.byKey[ipKey(itemID, personID, role)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return ip, nil
}

func (f *fakeItemPersonService) Update(_ context.Context, ip *domain.ItemPerson) (*domain.ItemPerson, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byKey[ipKey(ip.ItemID, ip.PersonID, ip.Role)] = ip
	return ip, nil
}

func (f *fakeItemPersonService) Delete(_ context.Context, itemID, personID, role string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byKey, ipKey(itemID, personID, role))
	return nil
}

func (f *fakeItemPersonService) List(_ context.Context, _, _ string, _ int, _ string) ([]*domain.ItemPerson, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	rows := make([]*domain.ItemPerson, 0, len(f.byKey))
	for _, ip := range f.byKey {
		rows = append(rows, ip)
	}
	return rows, "", nil
}

func validProtoItemPerson(itemID, personID, role string) *v1.ItemPerson {
	return &v1.ItemPerson{ItemId: itemID, PersonId: personID, Role: role}
}

func TestItemPersonHandler_CreateItemPerson(t *testing.T) {
	t.Run("valid request returns the created credit", func(t *testing.T) {
		svc := newFakeItemPersonService()
		h := apiconnect.NewItemPersonHandler(svc, nil)

		res, err := h.CreateItemPerson(context.Background(), connect.NewRequest(&v1.CreateItemPersonRequest{ItemPerson: validProtoItemPerson("i1", "p1", "performer")}))
		if err != nil {
			t.Fatalf("CreateItemPerson returned error: %v", err)
		}
		if res.Msg.GetItemPerson().GetRole() != "performer" {
			t.Fatalf("CreateItemPerson returned Role %q, want %q", res.Msg.GetItemPerson().GetRole(), "performer")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeItemPersonService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Role", Rule: "required", Value: ""}}}
		h := apiconnect.NewItemPersonHandler(svc, nil)

		_, err := h.CreateItemPerson(context.Background(), connect.NewRequest(&v1.CreateItemPersonRequest{ItemPerson: validProtoItemPerson("i1", "p1", "performer")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateItemPerson with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestItemPersonHandler_GetItemPerson(t *testing.T) {
	svc := newFakeItemPersonService()
	h := apiconnect.NewItemPersonHandler(svc, nil)
	svc.byKey[ipKey("i1", "p1", "performer")] = &domain.ItemPerson{ItemID: "i1", PersonID: "p1", Role: "performer", CreditedAs: "Existing"}

	res, err := h.GetItemPerson(context.Background(), connect.NewRequest(&v1.GetItemPersonRequest{ItemId: "i1", PersonId: "p1", Role: "performer"}))
	if err != nil {
		t.Fatalf("GetItemPerson returned error: %v", err)
	}
	if res.Msg.GetItemPerson().GetCreditedAs() != "Existing" {
		t.Fatalf("GetItemPerson returned CreditedAs %q, want %q", res.Msg.GetItemPerson().GetCreditedAs(), "Existing")
	}

	_, err = h.GetItemPerson(context.Background(), connect.NewRequest(&v1.GetItemPersonRequest{ItemId: "i1", PersonId: "missing", Role: "performer"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetItemPerson on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestItemPersonHandler_UpdateItemPerson(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeItemPersonService()
		h := apiconnect.NewItemPersonHandler(svc, nil)
		svc.byKey[ipKey("i1", "p1", "performer")] = &domain.ItemPerson{
			ItemID: "i1", PersonID: "p1", Role: "performer", CreditedAs: "Original", Character: "Original Character",
		}

		req := &v1.UpdateItemPersonRequest{
			ItemPerson: &v1.ItemPerson{ItemId: "i1", PersonId: "p1", Role: "performer", CreditedAs: "New Credit", Character: "Should be ignored"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"credited_as"}},
		}
		res, err := h.UpdateItemPerson(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateItemPerson returned error: %v", err)
		}
		if res.Msg.GetItemPerson().GetCreditedAs() != "New Credit" {
			t.Fatalf("UpdateItemPerson applied CreditedAs %q, want %q", res.Msg.GetItemPerson().GetCreditedAs(), "New Credit")
		}
		if res.Msg.GetItemPerson().GetCharacter() != "Original Character" {
			t.Fatalf("UpdateItemPerson touched Character: got %q", res.Msg.GetItemPerson().GetCharacter())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeItemPersonService()
		h := apiconnect.NewItemPersonHandler(svc, nil)

		_, err := h.UpdateItemPerson(context.Background(), connect.NewRequest(&v1.UpdateItemPersonRequest{ItemPerson: validProtoItemPerson("i1", "missing", "performer")}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateItemPerson on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestItemPersonHandler_DeleteItemPerson(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeItemPersonService()
		h := apiconnect.NewItemPersonHandler(svc, nil)
		svc.byKey[ipKey("i1", "p1", "performer")] = &domain.ItemPerson{ItemID: "i1", PersonID: "p1", Role: "performer"}

		req := &v1.DeleteItemPersonRequest{ItemId: "i1", PersonId: "p1", Role: "performer"}
		if _, err := h.DeleteItemPerson(context.Background(), connect.NewRequest(req)); err != nil {
			t.Fatalf("DeleteItemPerson returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemPersonService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewItemPersonHandler(svc, nil)

		_, err := h.DeleteItemPerson(context.Background(), connect.NewRequest(&v1.DeleteItemPersonRequest{ItemId: "i1", PersonId: "missing", Role: "performer"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteItemPerson on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestItemPersonHandler_ListItemPeople(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeItemPersonService()
		h := apiconnect.NewItemPersonHandler(svc, nil)
		svc.byKey[ipKey("i1", "p1", "performer")] = &domain.ItemPerson{ItemID: "i1", PersonID: "p1", Role: "performer"}
		svc.byKey[ipKey("i1", "p2", "performer")] = &domain.ItemPerson{ItemID: "i1", PersonID: "p2", Role: "performer"}

		res, err := h.ListItemPeople(context.Background(), connect.NewRequest(&v1.ListItemPeopleRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListItemPeople returned error: %v", err)
		}
		if len(res.Msg.GetItemPeople()) != 2 {
			t.Fatalf("ListItemPeople returned %d rows, want 2", len(res.Msg.GetItemPeople()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeItemPersonService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewItemPersonHandler(svc, nil)

		_, err := h.ListItemPeople(context.Background(), connect.NewRequest(&v1.ListItemPeopleRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListItemPeople with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
