package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

// fakePersonService is a test double for the personService interface
// PersonHandler depends on — per ADR 0003, handler tests run against a
// faked service, never a real one.
type fakePersonService struct {
	byID      map[string]*domain.Person
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakePersonService() *fakePersonService {
	return &fakePersonService{byID: make(map[string]*domain.Person)}
}

func (f *fakePersonService) Create(_ context.Context, p *domain.Person) (*domain.Person, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[p.ID] = p
	return p, nil
}

func (f *fakePersonService) Get(_ context.Context, id string) (*domain.Person, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return p, nil
}

func (f *fakePersonService) Update(_ context.Context, p *domain.Person) (*domain.Person, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[p.ID] = p
	return p, nil
}

func (f *fakePersonService) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, id)
	return nil
}

func (f *fakePersonService) List(_ context.Context, _ int, _ string) ([]*domain.Person, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	people := make([]*domain.Person, 0, len(f.byID))
	for _, p := range f.byID {
		people = append(people, p)
	}
	return people, "", nil
}

func validProtoPerson(id string) *v1.Person {
	return &v1.Person{
		Id:          id,
		Name:        "Test Person",
		Gender:      v1.Gender_GENDER_UNKNOWN,
		MonitorMode: v1.MonitorMode_MONITOR_MODE_NONE,
	}
}

func TestPersonHandler_CreatePerson(t *testing.T) {
	t.Run("valid request returns the created person", func(t *testing.T) {
		svc := newFakePersonService()
		h := apiconnect.NewPersonHandler(svc, nil)

		res, err := h.CreatePerson(context.Background(), connect.NewRequest(&v1.CreatePersonRequest{Person: validProtoPerson("p1")}))
		if err != nil {
			t.Fatalf("CreatePerson returned error: %v", err)
		}
		if res.Msg.GetPerson().GetId() != "p1" {
			t.Fatalf("CreatePerson returned ID %q, want %q", res.Msg.GetPerson().GetId(), "p1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		// Validation itself happens in internal/service.PersonService, not
		// the handler — per ADR 0003, this test only asserts the handler's
		// error-mapping behavior, so the fake simulates what the real
		// service returns for invalid input.
		svc := newFakePersonService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Name", Rule: "required", Value: ""}}}
		h := apiconnect.NewPersonHandler(svc, nil)

		_, err := h.CreatePerson(context.Background(), connect.NewRequest(&v1.CreatePersonRequest{Person: validProtoPerson("p1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreatePerson with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestPersonHandler_GetPerson(t *testing.T) {
	svc := newFakePersonService()
	h := apiconnect.NewPersonHandler(svc, nil)
	svc.byID["p1"] = &domain.Person{ID: "p1", Name: "Existing", Gender: domain.GenderUnknown, MonitorMode: domain.MonitorModeNone}

	res, err := h.GetPerson(context.Background(), connect.NewRequest(&v1.GetPersonRequest{Id: "p1"}))
	if err != nil {
		t.Fatalf("GetPerson returned error: %v", err)
	}
	if res.Msg.GetPerson().GetName() != "Existing" {
		t.Fatalf("GetPerson returned Name %q, want %q", res.Msg.GetPerson().GetName(), "Existing")
	}

	_, err = h.GetPerson(context.Background(), connect.NewRequest(&v1.GetPersonRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetPerson on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestPersonHandler_UpdatePerson_FieldMask(t *testing.T) {
	svc := newFakePersonService()
	h := apiconnect.NewPersonHandler(svc, nil)

	created := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.byID["p1"] = &domain.Person{
		ID: "p1", Name: "Original Name", Overview: "Original Overview",
		Gender: domain.GenderUnknown, MonitorMode: domain.MonitorModeNone,
		AddedAt: created, UpdatedAt: created,
	}

	req := &v1.UpdatePersonRequest{
		Person:     &v1.Person{Id: "p1", Name: "New Name", Overview: "Should be ignored"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
	}

	res, err := h.UpdatePerson(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("UpdatePerson returned error: %v", err)
	}
	if res.Msg.GetPerson().GetName() != "New Name" {
		t.Fatalf("UpdatePerson applied Name %q, want %q", res.Msg.GetPerson().GetName(), "New Name")
	}
	if res.Msg.GetPerson().GetOverview() != "Original Overview" {
		t.Fatalf("UpdatePerson with a name-only mask changed Overview to %q, want it untouched", res.Msg.GetPerson().GetOverview())
	}
}

func TestPersonHandler_DeletePerson(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakePersonService()
		h := apiconnect.NewPersonHandler(svc, nil)
		svc.byID["p1"] = &domain.Person{ID: "p1"}

		if _, err := h.DeletePerson(context.Background(), connect.NewRequest(&v1.DeletePersonRequest{Id: "p1"})); err != nil {
			t.Fatalf("DeletePerson returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakePersonService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewPersonHandler(svc, nil)

		_, err := h.DeletePerson(context.Background(), connect.NewRequest(&v1.DeletePersonRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeletePerson on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestPersonHandler_ListPeople(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakePersonService()
		h := apiconnect.NewPersonHandler(svc, nil)
		svc.byID["p1"] = &domain.Person{ID: "p1"}
		svc.byID["p2"] = &domain.Person{ID: "p2"}

		res, err := h.ListPeople(context.Background(), connect.NewRequest(&v1.ListPeopleRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListPeople returned error: %v", err)
		}
		if len(res.Msg.GetPeople()) != 2 {
			t.Fatalf("ListPeople returned %d people, want 2", len(res.Msg.GetPeople()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakePersonService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewPersonHandler(svc, nil)

		_, err := h.ListPeople(context.Background(), connect.NewRequest(&v1.ListPeopleRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListPeople with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestPersonHandler_UpdatePerson_ServiceErrorAfterFetch(t *testing.T) {
	svc := newFakePersonService()
	svc.byID["p1"] = &domain.Person{ID: "p1", Name: "Original", Gender: domain.GenderUnknown, MonitorMode: domain.MonitorModeNone}
	svc.updateErr = ports.ErrConflict
	h := apiconnect.NewPersonHandler(svc, nil)

	req := &v1.UpdatePersonRequest{Person: &v1.Person{Id: "p1", Name: "New"}}
	_, err := h.UpdatePerson(context.Background(), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("UpdatePerson with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
	}
}

func TestPersonHandler_UnmappedErrorBecomesInternal(t *testing.T) {
	svc := newFakePersonService()
	svc.getErr = errors.New("boom")
	h := apiconnect.NewPersonHandler(svc, nil)

	_, err := h.GetPerson(context.Background(), connect.NewRequest(&v1.GetPersonRequest{Id: "p1"}))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("GetPerson with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
	}
}
