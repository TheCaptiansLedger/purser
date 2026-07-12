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

type fakeEntryPersonService struct {
	byKey     map[string]*domain.EntryPerson
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeEntryPersonService() *fakeEntryPersonService {
	return &fakeEntryPersonService{byKey: make(map[string]*domain.EntryPerson)}
}

func epKey(libraryEntryID, personID, role string) string {
	return libraryEntryID + "|" + personID + "|" + role
}

func (f *fakeEntryPersonService) Create(_ context.Context, ep *domain.EntryPerson) (*domain.EntryPerson, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byKey[epKey(ep.LibraryEntryID, ep.PersonID, ep.Role)] = ep
	return ep, nil
}

func (f *fakeEntryPersonService) Get(_ context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	ep, ok := f.byKey[epKey(libraryEntryID, personID, role)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return ep, nil
}

func (f *fakeEntryPersonService) Update(_ context.Context, ep *domain.EntryPerson) (*domain.EntryPerson, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byKey[epKey(ep.LibraryEntryID, ep.PersonID, ep.Role)] = ep
	return ep, nil
}

func (f *fakeEntryPersonService) Delete(_ context.Context, libraryEntryID, personID, role string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byKey, epKey(libraryEntryID, personID, role))
	return nil
}

func (f *fakeEntryPersonService) List(_ context.Context, _, _ string, _ int, _ string) ([]*domain.EntryPerson, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	rows := make([]*domain.EntryPerson, 0, len(f.byKey))
	for _, ep := range f.byKey {
		rows = append(rows, ep)
	}
	return rows, "", nil
}

func validProtoEntryPerson(libraryEntryID, personID, role string) *v1.EntryPerson {
	return &v1.EntryPerson{LibraryEntryId: libraryEntryID, PersonId: personID, Role: role}
}

func TestEntryPersonHandler_CreateEntryPerson(t *testing.T) {
	t.Run("valid request returns the created credit", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		h := apiconnect.NewEntryPersonHandler(svc, nil)

		res, err := h.CreateEntryPerson(context.Background(), connect.NewRequest(&v1.CreateEntryPersonRequest{EntryPerson: validProtoEntryPerson("e1", "p1", "director")}))
		if err != nil {
			t.Fatalf("CreateEntryPerson returned error: %v", err)
		}
		if res.Msg.GetEntryPerson().GetRole() != "director" {
			t.Fatalf("CreateEntryPerson returned Role %q, want %q", res.Msg.GetEntryPerson().GetRole(), "director")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Role", Rule: "required", Value: ""}}}
		h := apiconnect.NewEntryPersonHandler(svc, nil)

		_, err := h.CreateEntryPerson(context.Background(), connect.NewRequest(&v1.CreateEntryPersonRequest{EntryPerson: validProtoEntryPerson("e1", "p1", "director")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateEntryPerson with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestEntryPersonHandler_GetEntryPerson(t *testing.T) {
	svc := newFakeEntryPersonService()
	h := apiconnect.NewEntryPersonHandler(svc, nil)
	svc.byKey[epKey("e1", "p1", "director")] = &domain.EntryPerson{LibraryEntryID: "e1", PersonID: "p1", Role: "director", CreditedAs: "Existing"}

	res, err := h.GetEntryPerson(context.Background(), connect.NewRequest(&v1.GetEntryPersonRequest{LibraryEntryId: "e1", PersonId: "p1", Role: "director"}))
	if err != nil {
		t.Fatalf("GetEntryPerson returned error: %v", err)
	}
	if res.Msg.GetEntryPerson().GetCreditedAs() != "Existing" {
		t.Fatalf("GetEntryPerson returned CreditedAs %q, want %q", res.Msg.GetEntryPerson().GetCreditedAs(), "Existing")
	}

	_, err = h.GetEntryPerson(context.Background(), connect.NewRequest(&v1.GetEntryPersonRequest{LibraryEntryId: "e1", PersonId: "missing", Role: "director"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetEntryPerson on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestEntryPersonHandler_UpdateEntryPerson(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		h := apiconnect.NewEntryPersonHandler(svc, nil)
		svc.byKey[epKey("e1", "p1", "director")] = &domain.EntryPerson{
			LibraryEntryID: "e1", PersonID: "p1", Role: "director",
			CreditedAs: "Original", Character: "Original Character",
		}

		req := &v1.UpdateEntryPersonRequest{
			EntryPerson: &v1.EntryPerson{LibraryEntryId: "e1", PersonId: "p1", Role: "director", CreditedAs: "New Credit", Character: "Should be ignored"},
			UpdateMask:  &fieldmaskpb.FieldMask{Paths: []string{"credited_as"}},
		}
		res, err := h.UpdateEntryPerson(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateEntryPerson returned error: %v", err)
		}
		if res.Msg.GetEntryPerson().GetCreditedAs() != "New Credit" {
			t.Fatalf("UpdateEntryPerson applied CreditedAs %q, want %q", res.Msg.GetEntryPerson().GetCreditedAs(), "New Credit")
		}
		if res.Msg.GetEntryPerson().GetCharacter() != "Original Character" {
			t.Fatalf("UpdateEntryPerson touched Character: got %q", res.Msg.GetEntryPerson().GetCharacter())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		h := apiconnect.NewEntryPersonHandler(svc, nil)

		_, err := h.UpdateEntryPerson(context.Background(), connect.NewRequest(&v1.UpdateEntryPersonRequest{EntryPerson: validProtoEntryPerson("e1", "missing", "director")}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateEntryPerson on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestEntryPersonHandler_DeleteEntryPerson(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		h := apiconnect.NewEntryPersonHandler(svc, nil)
		svc.byKey[epKey("e1", "p1", "director")] = &domain.EntryPerson{LibraryEntryID: "e1", PersonID: "p1", Role: "director"}

		req := &v1.DeleteEntryPersonRequest{LibraryEntryId: "e1", PersonId: "p1", Role: "director"}
		if _, err := h.DeleteEntryPerson(context.Background(), connect.NewRequest(req)); err != nil {
			t.Fatalf("DeleteEntryPerson returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewEntryPersonHandler(svc, nil)

		_, err := h.DeleteEntryPerson(context.Background(), connect.NewRequest(&v1.DeleteEntryPersonRequest{LibraryEntryId: "e1", PersonId: "missing", Role: "director"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteEntryPerson on missing key returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestEntryPersonHandler_ListEntryPeople(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		h := apiconnect.NewEntryPersonHandler(svc, nil)
		svc.byKey[epKey("e1", "p1", "director")] = &domain.EntryPerson{LibraryEntryID: "e1", PersonID: "p1", Role: "director"}
		svc.byKey[epKey("e1", "p2", "writer")] = &domain.EntryPerson{LibraryEntryID: "e1", PersonID: "p2", Role: "writer"}

		res, err := h.ListEntryPeople(context.Background(), connect.NewRequest(&v1.ListEntryPeopleRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListEntryPeople returned error: %v", err)
		}
		if len(res.Msg.GetEntryPeople()) != 2 {
			t.Fatalf("ListEntryPeople returned %d rows, want 2", len(res.Msg.GetEntryPeople()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeEntryPersonService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewEntryPersonHandler(svc, nil)

		_, err := h.ListEntryPeople(context.Background(), connect.NewRequest(&v1.ListEntryPeopleRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListEntryPeople with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
