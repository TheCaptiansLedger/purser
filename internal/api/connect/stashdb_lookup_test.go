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

type fakeStashDBLookupService struct {
	gotID           string
	returnPerformer *ports.Performer
	performerErr    error

	gotTerm          string
	returnPerformers []ports.Performer
	searchErr        error
}

func (f *fakeStashDBLookupService) LookupPerformer(_ context.Context, id string) (*ports.Performer, error) {
	f.gotID = id
	if f.performerErr != nil {
		return nil, f.performerErr
	}
	return f.returnPerformer, nil
}

func (f *fakeStashDBLookupService) SearchPerformers(_ context.Context, term string) ([]ports.Performer, error) {
	f.gotTerm = term
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.returnPerformers, nil
}

func TestStashDBHandler_LookupPerformer(t *testing.T) {
	t.Run("valid request maps performer fields and images correctly", func(t *testing.T) {
		svc := &fakeStashDBLookupService{returnPerformer: &ports.Performer{
			ID:     "performer-1",
			Name:   "Jane Doe",
			Images: []ports.Image{{ID: "img-1", URL: "https://example.invalid/a.jpg", Width: 800, Height: 1200}},
			URLs:   []ports.URL{{URL: "https://twitter.com/jane", Site: ports.Site{ID: "site-1", Name: "Twitter"}}},
		}}
		h := apiconnect.NewStashDBHandler(svc, nil)

		res, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupStashDBPerformerRequest{Id: "performer-1"}))
		if err != nil {
			t.Fatalf("LookupPerformer returned error: %v", err)
		}
		got := res.Msg.GetPerformer()
		if got.GetId() != "performer-1" || got.GetName() != "Jane Doe" {
			t.Errorf("LookupPerformer = %+v, want Id=performer-1 Name=Jane Doe", got)
		}
		if len(got.GetImages()) != 1 || got.GetImages()[0].GetUrl() != "https://example.invalid/a.jpg" {
			t.Errorf("Images = %+v, want one entry with the fake's URL", got.GetImages())
		}
		if len(got.GetUrls()) != 1 || got.GetUrls()[0].GetSite().GetName() != "Twitter" {
			t.Errorf("Urls = %+v, want one entry with Site.Name=Twitter", got.GetUrls())
		}
		if svc.gotID != "performer-1" {
			t.Errorf("LookupPerformer passed id=%q, want performer-1", svc.gotID)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeStashDBLookupService{performerErr: ports.ErrNotFound}
		h := apiconnect.NewStashDBHandler(svc, nil)

		_, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupStashDBPerformerRequest{Id: "x"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("LookupPerformer with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeStashDBLookupService{performerErr: errors.New("boom")}
		h := apiconnect.NewStashDBHandler(svc, nil)

		_, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupStashDBPerformerRequest{Id: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("LookupPerformer with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("no images maps to an empty, non-nil slice", func(t *testing.T) {
		svc := &fakeStashDBLookupService{returnPerformer: &ports.Performer{ID: "performer-2", Name: "No Photo"}}
		h := apiconnect.NewStashDBHandler(svc, nil)

		res, err := h.LookupPerformer(context.Background(), connect.NewRequest(&afterdarkv1.LookupStashDBPerformerRequest{Id: "performer-2"}))
		if err != nil {
			t.Fatalf("LookupPerformer returned error: %v", err)
		}
		if len(res.Msg.GetPerformer().GetImages()) != 0 {
			t.Errorf("Images = %+v, want empty", res.Msg.GetPerformer().GetImages())
		}
	})
}

func TestStashDBHandler_SearchPerformers(t *testing.T) {
	t.Run("valid request maps every returned performer", func(t *testing.T) {
		svc := &fakeStashDBLookupService{returnPerformers: []ports.Performer{
			{ID: "performer-1", Name: "Alex Coal"},
			{ID: "performer-2", Name: "Alex Coal (alt)"},
		}}
		h := apiconnect.NewStashDBHandler(svc, nil)

		res, err := h.SearchPerformers(context.Background(), connect.NewRequest(&afterdarkv1.SearchStashDBPerformersRequest{Term: "Alex Coal"}))
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
		svc := &fakeStashDBLookupService{searchErr: errors.New("boom")}
		h := apiconnect.NewStashDBHandler(svc, nil)

		_, err := h.SearchPerformers(context.Background(), connect.NewRequest(&afterdarkv1.SearchStashDBPerformersRequest{Term: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("SearchPerformers with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("zero results maps to an empty, non-nil slice", func(t *testing.T) {
		svc := &fakeStashDBLookupService{}
		h := apiconnect.NewStashDBHandler(svc, nil)

		res, err := h.SearchPerformers(context.Background(), connect.NewRequest(&afterdarkv1.SearchStashDBPerformersRequest{Term: "nobody"}))
		if err != nil {
			t.Fatalf("SearchPerformers returned error: %v", err)
		}
		if len(res.Msg.GetPerformers()) != 0 {
			t.Errorf("Performers = %+v, want empty", res.Msg.GetPerformers())
		}
	})
}
