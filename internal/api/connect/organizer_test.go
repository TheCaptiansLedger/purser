package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeOrganizerService struct {
	gotMediaFileID string
	returnMF       *domain.MediaFile
	err            error
}

func (f *fakeOrganizerService) Organize(_ context.Context, mediaFileID string) (*domain.MediaFile, error) {
	f.gotMediaFileID = mediaFileID
	if f.err != nil {
		return nil, f.err
	}
	return f.returnMF, nil
}

func TestOrganizerHandler_Organize(t *testing.T) {
	t.Run("valid request returns the organized media file", func(t *testing.T) {
		svc := &fakeOrganizerService{returnMF: &domain.MediaFile{ID: "mf-1", ItemID: "item-1", Path: "/library/Artist/Album/01 - Track.flac"}}
		h := apiconnect.NewOrganizerHandler(svc, nil)

		res, err := h.Organize(context.Background(), connect.NewRequest(&pipelinev1.OrganizeRequest{MediaFileId: "mf-1"}))
		if err != nil {
			t.Fatalf("Organize returned error: %v", err)
		}
		if res.Msg.GetMediaFile().GetId() != "mf-1" {
			t.Fatalf("Organize returned media_file.id %q, want %q", res.Msg.GetMediaFile().GetId(), "mf-1")
		}
		if res.Msg.GetMediaFile().GetPath() != "/library/Artist/Album/01 - Track.flac" {
			t.Fatalf("Organize returned media_file.path %q, want the organized path", res.Msg.GetMediaFile().GetPath())
		}
		if svc.gotMediaFileID != "mf-1" {
			t.Fatalf("Organize passed media_file_id %q, want %q", svc.gotMediaFileID, "mf-1")
		}
	})

	t.Run("a not-found-shaped error maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeOrganizerService{err: ports.ErrNotFound}
		h := apiconnect.NewOrganizerHandler(svc, nil)

		_, err := h.Organize(context.Background(), connect.NewRequest(&pipelinev1.OrganizeRequest{MediaFileId: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("Organize with ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("a destination-collision error maps to CodeAlreadyExists", func(t *testing.T) {
		svc := &fakeOrganizerService{err: ports.ErrDestinationExists}
		h := apiconnect.NewOrganizerHandler(svc, nil)

		_, err := h.Organize(context.Background(), connect.NewRequest(&pipelinev1.OrganizeRequest{MediaFileId: "mf-1"}))
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("Organize with ErrDestinationExists returned code %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
		}
	})

	t.Run("a destination-outside-root error maps to CodeFailedPrecondition", func(t *testing.T) {
		svc := &fakeOrganizerService{err: ports.ErrDestinationOutsideRoot}
		h := apiconnect.NewOrganizerHandler(svc, nil)

		_, err := h.Organize(context.Background(), connect.NewRequest(&pipelinev1.OrganizeRequest{MediaFileId: "mf-1"}))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("Organize with ErrDestinationOutsideRoot returned code %v, want %v", connect.CodeOf(err), connect.CodeFailedPrecondition)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeOrganizerService{err: errors.New("boom")}
		h := apiconnect.NewOrganizerHandler(svc, nil)

		_, err := h.Organize(context.Background(), connect.NewRequest(&pipelinev1.OrganizeRequest{MediaFileId: "mf-1"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("Organize with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
