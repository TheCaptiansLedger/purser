package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
	apiconnect "purser/internal/api/connect"
)

type fakePerformerProfileService struct {
	byID      map[string]*afterdark.PerformerProfile
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakePerformerProfileService() *fakePerformerProfileService {
	return &fakePerformerProfileService{byID: make(map[string]*afterdark.PerformerProfile)}
}

func (f *fakePerformerProfileService) Create(_ context.Context, p *afterdark.PerformerProfile) (*afterdark.PerformerProfile, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[p.PersonID] = p
	return p, nil
}

func (f *fakePerformerProfileService) Get(_ context.Context, personID string) (*afterdark.PerformerProfile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.byID[personID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return p, nil
}

func (f *fakePerformerProfileService) Update(_ context.Context, p *afterdark.PerformerProfile) (*afterdark.PerformerProfile, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[p.PersonID] = p
	return p, nil
}

func (f *fakePerformerProfileService) Delete(_ context.Context, personID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, personID)
	return nil
}

func (f *fakePerformerProfileService) List(_ context.Context, _ int, _ string) ([]*afterdark.PerformerProfile, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	profiles := make([]*afterdark.PerformerProfile, 0, len(f.byID))
	for _, p := range f.byID {
		profiles = append(profiles, p)
	}
	return profiles, "", nil
}

func validProtoPerformerProfile(personID string) *afterdarkv1.PerformerProfile {
	return &afterdarkv1.PerformerProfile{PersonId: personID, CupSize: "34"}
}

func TestPerformerProfileHandler_CreatePerformerProfile(t *testing.T) {
	t.Run("valid request returns the created profile", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		h := apiconnect.NewPerformerProfileHandler(svc, nil)

		res, err := h.CreatePerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.CreatePerformerProfileRequest{PerformerProfile: validProtoPerformerProfile("p1")}))
		if err != nil {
			t.Fatalf("CreatePerformerProfile returned error: %v", err)
		}
		if res.Msg.GetPerformerProfile().GetPersonId() != "p1" {
			t.Fatalf("CreatePerformerProfile returned PersonId %q, want %q", res.Msg.GetPerformerProfile().GetPersonId(), "p1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "PersonID", Rule: "required", Value: ""}}}
		h := apiconnect.NewPerformerProfileHandler(svc, nil)

		_, err := h.CreatePerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.CreatePerformerProfileRequest{PerformerProfile: validProtoPerformerProfile("p1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreatePerformerProfile with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestPerformerProfileHandler_GetPerformerProfile(t *testing.T) {
	svc := newFakePerformerProfileService()
	h := apiconnect.NewPerformerProfileHandler(svc, nil)
	svc.byID["p1"] = &afterdark.PerformerProfile{PersonID: "p1", CupSize: "Existing"}

	res, err := h.GetPerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.GetPerformerProfileRequest{PersonId: "p1"}))
	if err != nil {
		t.Fatalf("GetPerformerProfile returned error: %v", err)
	}
	if res.Msg.GetPerformerProfile().GetCupSize() != "Existing" {
		t.Fatalf("GetPerformerProfile returned CupSize %q, want %q", res.Msg.GetPerformerProfile().GetCupSize(), "Existing")
	}

	_, err = h.GetPerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.GetPerformerProfileRequest{PersonId: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetPerformerProfile on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestPerformerProfileHandler_UpdatePerformerProfile(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		h := apiconnect.NewPerformerProfileHandler(svc, nil)
		svc.byID["p1"] = &afterdark.PerformerProfile{PersonID: "p1", CupSize: "Original", BandSize: "Original Band"}

		req := &afterdarkv1.UpdatePerformerProfileRequest{
			PerformerProfile: &afterdarkv1.PerformerProfile{PersonId: "p1", CupSize: "New", BandSize: "Should be ignored"},
			UpdateMask:       &fieldmaskpb.FieldMask{Paths: []string{"cup_size"}},
		}
		res, err := h.UpdatePerformerProfile(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdatePerformerProfile returned error: %v", err)
		}
		if res.Msg.GetPerformerProfile().GetCupSize() != "New" {
			t.Fatalf("UpdatePerformerProfile applied CupSize %q, want %q", res.Msg.GetPerformerProfile().GetCupSize(), "New")
		}
		if res.Msg.GetPerformerProfile().GetBandSize() != "Original Band" {
			t.Fatalf("UpdatePerformerProfile touched BandSize: got %q", res.Msg.GetPerformerProfile().GetBandSize())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		h := apiconnect.NewPerformerProfileHandler(svc, nil)

		_, err := h.UpdatePerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.UpdatePerformerProfileRequest{PerformerProfile: &afterdarkv1.PerformerProfile{PersonId: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdatePerformerProfile on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("update failure maps through mapError", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		svc.byID["p1"] = &afterdark.PerformerProfile{PersonID: "p1"}
		svc.updateErr = ports.ErrConflict
		h := apiconnect.NewPerformerProfileHandler(svc, nil)

		_, err := h.UpdatePerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.UpdatePerformerProfileRequest{PerformerProfile: &afterdarkv1.PerformerProfile{PersonId: "p1", CupSize: "New"}}))
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("UpdatePerformerProfile with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})
}

func TestPerformerProfileHandler_DeletePerformerProfile(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		h := apiconnect.NewPerformerProfileHandler(svc, nil)
		svc.byID["p1"] = &afterdark.PerformerProfile{PersonID: "p1"}

		if _, err := h.DeletePerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.DeletePerformerProfileRequest{PersonId: "p1"})); err != nil {
			t.Fatalf("DeletePerformerProfile returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewPerformerProfileHandler(svc, nil)

		_, err := h.DeletePerformerProfile(context.Background(), connect.NewRequest(&afterdarkv1.DeletePerformerProfileRequest{PersonId: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeletePerformerProfile on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestPerformerProfileHandler_ListPerformerProfiles(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		h := apiconnect.NewPerformerProfileHandler(svc, nil)
		svc.byID["p1"] = &afterdark.PerformerProfile{PersonID: "p1"}
		svc.byID["p2"] = &afterdark.PerformerProfile{PersonID: "p2"}

		res, err := h.ListPerformerProfiles(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformerProfilesRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListPerformerProfiles returned error: %v", err)
		}
		if len(res.Msg.GetPerformerProfiles()) != 2 {
			t.Fatalf("ListPerformerProfiles returned %d profiles, want 2", len(res.Msg.GetPerformerProfiles()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakePerformerProfileService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewPerformerProfileHandler(svc, nil)

		_, err := h.ListPerformerProfiles(context.Background(), connect.NewRequest(&afterdarkv1.ListPerformerProfilesRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListPerformerProfiles with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
