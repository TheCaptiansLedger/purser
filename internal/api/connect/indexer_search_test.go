package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"testing"
	"time"

	"connectrpc.com/connect"

	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeIndexerSearchService struct {
	gotParams      ports.IndexerSearchParams
	returnReleases []ports.IndexerRelease
	err            error
}

func (f *fakeIndexerSearchService) Search(_ context.Context, params ports.IndexerSearchParams) ([]ports.IndexerRelease, error) {
	f.gotParams = params
	if f.err != nil {
		return nil, f.err
	}
	return f.returnReleases, nil
}

func TestIndexerSearchHandler_Search(t *testing.T) {
	t.Run("valid request returns releases and passes params through", func(t *testing.T) {
		publishDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		svc := &fakeIndexerSearchService{returnReleases: []ports.IndexerRelease{
			{
				GUID: "release-1", Title: "Some Release", IndexerName: "fixture-indexer",
				Size: 1024, Protocol: ports.ProtocolTorrent, PublishDate: publishDate,
				Seeders: 10, Leechers: 2,
				DownloadURL: "http://example.invalid/download", MagnetURL: "magnet:?xt=urn:btih:abc",
				InfoURL: "http://example.invalid/info", InfoHash: "abc123",
				Categories: []ports.Category{{ID: 3000, Name: "Music"}},
			},
		}}
		h := apiconnect.NewIndexerSearchHandler(svc, nil)

		res, err := h.Search(context.Background(), connect.NewRequest(&acquisitionv1.SearchIndexersRequest{
			Query:      "some release",
			Categories: []int32{3000},
			IndexerIds: []int32{1},
		}))
		if err != nil {
			t.Fatalf("Search returned error: %v", err)
		}
		if len(res.Msg.GetReleases()) != 1 {
			t.Fatalf("Search returned %d releases, want 1", len(res.Msg.GetReleases()))
		}
		got := res.Msg.GetReleases()[0]
		if got.GetGuid() != "release-1" || got.GetTitle() != "Some Release" || got.GetIndexerName() != "fixture-indexer" {
			t.Errorf("release = %+v, want guid=release-1 title=%q indexer_name=fixture-indexer", got, "Some Release")
		}
		if got.GetProtocol() != acquisitionv1.Protocol_PROTOCOL_TORRENT {
			t.Errorf("Protocol = %v, want PROTOCOL_TORRENT", got.GetProtocol())
		}
		if got.GetSeeders() != 10 || got.GetLeechers() != 2 {
			t.Errorf("Seeders/Leechers = %d/%d, want 10/2", got.GetSeeders(), got.GetLeechers())
		}
		if len(got.GetCategories()) != 1 || got.GetCategories()[0].GetId() != 3000 || got.GetCategories()[0].GetName() != "Music" {
			t.Errorf("Categories = %+v, want one {id:3000 name:Music}", got.GetCategories())
		}
		if !got.GetPublishDate().AsTime().Equal(publishDate) {
			t.Errorf("PublishDate = %v, want %v", got.GetPublishDate().AsTime(), publishDate)
		}

		if svc.gotParams.Query != "some release" || len(svc.gotParams.Categories) != 1 || svc.gotParams.Categories[0] != 3000 {
			t.Errorf("Search passed params=%+v, want query=%q categories=[3000]", svc.gotParams, "some release")
		}
		if len(svc.gotParams.IndexerIDs) != 1 || svc.gotParams.IndexerIDs[0] != 1 {
			t.Errorf("Search passed indexer_ids=%+v, want [1]", svc.gotParams.IndexerIDs)
		}
	})

	t.Run("empty result is a successful empty list, not an error", func(t *testing.T) {
		svc := &fakeIndexerSearchService{returnReleases: []ports.IndexerRelease{}}
		h := apiconnect.NewIndexerSearchHandler(svc, nil)

		res, err := h.Search(context.Background(), connect.NewRequest(&acquisitionv1.SearchIndexersRequest{Query: "no-such-release"}))
		if err != nil {
			t.Fatalf("Search returned error: %v, want nil for a zero-result search", err)
		}
		if len(res.Msg.GetReleases()) != 0 {
			t.Errorf("Search returned %d releases, want 0", len(res.Msg.GetReleases()))
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeIndexerSearchService{err: errors.New("boom")}
		h := apiconnect.NewIndexerSearchHandler(svc, nil)

		_, err := h.Search(context.Background(), connect.NewRequest(&acquisitionv1.SearchIndexersRequest{Query: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("Search with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
