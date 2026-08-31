package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeThePornDBLookupService struct {
	gotID           string
	returnPerformer *ports.TPDBPerformer
	performerErr    error

	gotTerm          string
	returnPerformers []ports.TPDBPerformer
	searchErr        error
}

func (f *fakeThePornDBLookupService) LookupPerformer(_ context.Context, id string) (*ports.TPDBPerformer, error) {
	f.gotID = id
	if f.performerErr != nil {
		return nil, f.performerErr
	}
	return f.returnPerformer, nil
}

func (f *fakeThePornDBLookupService) SearchPerformers(_ context.Context, term string) ([]ports.TPDBPerformer, error) {
	f.gotTerm = term
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.returnPerformers, nil
}

func TestThePornDBHandler_LookupPerformer(t *testing.T) {
	t.Run("valid request maps performer fields, posters, extras, and one level of parent", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{returnPerformer: &ports.TPDBPerformer{
			ID:       "performer-1",
			Name:     "Jane Doe",
			Image:    "https://example.invalid/full.jpg",
			Posters:  []ports.TPDBImage{{ID: 1, URL: "https://example.invalid/poster.jpg", Size: 500, Order: 1}},
			Extras:   ports.TPDBPerformerExtras{Gender: "Female", Links: ports.TPDBLinks{"IAFD": "https://iafd.com/x"}},
			IsParent: false,
			Parent:   &ports.TPDBPerformer{ID: "parent-1", Name: "Jane Doe (canonical)", IsParent: true},
		}}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		res, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupThePornDBPerformerRequest{Id: "performer-1"}))
		if err != nil {
			t.Fatalf("LookupPerformer returned error: %v", err)
		}
		got := res.Msg.GetPerformer()
		if got.GetId() != "performer-1" || got.GetName() != "Jane Doe" || got.GetImage() != "https://example.invalid/full.jpg" {
			t.Errorf("LookupPerformer = %+v, want the fake's fields", got)
		}
		if len(got.GetPosters()) != 1 || got.GetPosters()[0].GetUrl() != "https://example.invalid/poster.jpg" {
			t.Errorf("Posters = %+v, want one entry with the fake's URL", got.GetPosters())
		}
		if got.GetExtras().GetGender() != "Female" || got.GetExtras().GetLinks()["IAFD"] != "https://iafd.com/x" {
			t.Errorf("Extras = %+v, want Gender=Female and the IAFD link", got.GetExtras())
		}
		if got.GetParent().GetId() != "parent-1" || got.GetParent().GetName() != "Jane Doe (canonical)" {
			t.Errorf("Parent = %+v, want the fake's parent fields", got.GetParent())
		}
		if svc.gotID != "performer-1" {
			t.Errorf("LookupPerformer passed id=%q, want performer-1", svc.gotID)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{performerErr: ports.ErrNotFound}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		_, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupThePornDBPerformerRequest{Id: "x"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("LookupPerformer with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{performerErr: errors.New("boom")}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		_, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupThePornDBPerformerRequest{Id: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("LookupPerformer with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("no parent maps to nil, not a zero-value message", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{returnPerformer: &ports.TPDBPerformer{ID: "performer-2", Name: "Canonical", IsParent: true}}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		res, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupThePornDBPerformerRequest{Id: "performer-2"}))
		if err != nil {
			t.Fatalf("LookupPerformer returned error: %v", err)
		}
		if res.Msg.GetPerformer().GetParent() != nil {
			t.Errorf("Parent = %+v, want nil", res.Msg.GetPerformer().GetParent())
		}
	})
}

func TestThePornDBHandler_SearchPerformers(t *testing.T) {
	t.Run("valid request maps every returned performer", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{returnPerformers: []ports.TPDBPerformer{
			{ID: "performer-1", Name: "Alex Coal"},
			{ID: "performer-2", Name: "Alex Coal (alt)"},
		}}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		res, err := h.SearchPerformers(context.Background(), connect.NewRequest(&afterdarkv1.SearchThePornDBPerformersRequest{Term: "Alex Coal"}))
		if err != nil {
			t.Fatalf("SearchPerformers returned error: %v", err)
		}
		if len(res.Msg.GetPerformers()) != 2 {
			t.Fatalf("len(Performers) = %d, want 2", len(res.Msg.GetPerformers()))
		}
		if res.Msg.GetPerformers()[0].GetName() != "Alex Coal" || res.Msg.GetPerformers()[1].GetName() != "Alex Coal (alt)" {
			t.Errorf("Performers = %+v, want the fake's names in order", res.Msg.GetPerformers())
		}
		if svc.gotTerm != "Alex Coal" {
			t.Errorf("SearchPerformers passed term=%q, want Alex Coal", svc.gotTerm)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{searchErr: errors.New("boom")}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		_, err := h.SearchPerformers(context.Background(), connect.NewRequest(&afterdarkv1.SearchThePornDBPerformersRequest{Term: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("SearchPerformers with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("zero results maps to an empty, non-nil slice", func(t *testing.T) {
		svc := &fakeThePornDBLookupService{}
		h := apiconnect.NewThePornDBHandler(svc, nil)

		res, err := h.SearchPerformers(context.Background(), connect.NewRequest(&afterdarkv1.SearchThePornDBPerformersRequest{Term: "nobody"}))
		if err != nil {
			t.Fatalf("SearchPerformers returned error: %v", err)
		}
		if len(res.Msg.GetPerformers()) != 0 {
			t.Errorf("Performers = %+v, want empty", res.Msg.GetPerformers())
		}
	})
}
