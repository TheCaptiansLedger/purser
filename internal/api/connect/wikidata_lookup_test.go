package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeWikidataLookupService struct {
	gotEntityURL string
	returnImages []ports.WikidataImage
	imagesErr    error
}

func (f *fakeWikidataLookupService) LookupImage(_ context.Context, entityURL string) ([]ports.WikidataImage, error) {
	f.gotEntityURL = entityURL
	if f.imagesErr != nil {
		return nil, f.imagesErr
	}
	return f.returnImages, nil
}

func TestWikidataHandler_LookupImage(t *testing.T) {
	t.Run("valid request maps images correctly", func(t *testing.T) {
		svc := &fakeWikidataLookupService{returnImages: []ports.WikidataImage{
			{URL: "https://commons.wikimedia.org/wiki/Special:FilePath/REO_Speedwagon.jpg"},
			{URL: "https://commons.wikimedia.org/wiki/Special:FilePath/REO_Speedwagon_2.jpg"},
		}}
		h := apiconnect.NewWikidataHandler(svc, nil)

		res, err := h.LookupImage(context.Background(), connect.NewRequest(&musicv1.LookupWikidataImageRequest{Url: "https://www.wikidata.org/wiki/Q845084"}))
		if err != nil {
			t.Fatalf("LookupImage returned error: %v", err)
		}
		images := res.Msg.GetImages()
		if len(images) != 2 {
			t.Fatalf("len(Images) = %d, want 2", len(images))
		}
		if images[0].GetUrl() != "https://commons.wikimedia.org/wiki/Special:FilePath/REO_Speedwagon.jpg" {
			t.Errorf("Images[0].Url = %q, want the fake's first URL", images[0].GetUrl())
		}
		if svc.gotEntityURL != "https://www.wikidata.org/wiki/Q845084" {
			t.Errorf("LookupImage passed url=%q, want the Q845084 entity URL", svc.gotEntityURL)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeWikidataLookupService{imagesErr: ports.ErrNotFound}
		h := apiconnect.NewWikidataHandler(svc, nil)

		_, err := h.LookupImage(context.Background(), connect.NewRequest(&musicv1.LookupWikidataImageRequest{Url: "x"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("LookupImage with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeWikidataLookupService{imagesErr: errors.New("boom")}
		h := apiconnect.NewWikidataHandler(svc, nil)

		_, err := h.LookupImage(context.Background(), connect.NewRequest(&musicv1.LookupWikidataImageRequest{Url: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("LookupImage with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("no images maps to an empty, non-nil slice", func(t *testing.T) {
		svc := &fakeWikidataLookupService{}
		h := apiconnect.NewWikidataHandler(svc, nil)

		res, err := h.LookupImage(context.Background(), connect.NewRequest(&musicv1.LookupWikidataImageRequest{Url: "x"}))
		if err != nil {
			t.Fatalf("LookupImage returned error: %v", err)
		}
		if len(res.Msg.GetImages()) != 0 {
			t.Errorf("Images = %+v, want empty", res.Msg.GetImages())
		}
	})
}
