package apiconnect_test

import (
	"context"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeMusicReleaseService struct {
	byID      map[string]*music.Release
	createErr error
	getErr    error
}

func newFakeMusicReleaseService() *fakeMusicReleaseService {
	return &fakeMusicReleaseService{byID: make(map[string]*music.Release)}
}

func (f *fakeMusicReleaseService) Create(_ context.Context, r *music.Release) (*music.Release, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	r.ID = "server-generated-id"
	f.byID[r.ID] = r
	return r, nil
}

func (f *fakeMusicReleaseService) Get(_ context.Context, id string) (*music.Release, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	r, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return r, nil
}

func validProtoMusicRelease() *musicv1.Release {
	return &musicv1.Release{
		Id:             "caller-supplied-id",
		GroupId:        "group1",
		LibraryEntryId: "entry1",
		Title:          "Hi Infidelity (2024 Remaster)",
		Status:         musicv1.ReleaseStatus_RELEASE_STATUS_STUB,
	}
}

func TestMusicReleaseHandler_CreateMusicRelease(t *testing.T) {
	t.Run("valid request returns the created release with a server-assigned id", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)

		res, err := h.CreateMusicRelease(context.Background(), connect.NewRequest(&musicv1.CreateMusicReleaseRequest{MusicRelease: validProtoMusicRelease()}))
		if err != nil {
			t.Fatalf("CreateMusicRelease returned error: %v", err)
		}
		if res.Msg.GetMusicRelease().GetId() != "server-generated-id" {
			t.Fatalf("CreateMusicRelease returned Id %q, want the server-assigned id", res.Msg.GetMusicRelease().GetId())
		}
		if res.Msg.GetMusicRelease().GetTitle() != "Hi Infidelity (2024 Remaster)" {
			t.Fatalf("CreateMusicRelease returned Title %q, want %q", res.Msg.GetMusicRelease().GetTitle(), "Hi Infidelity (2024 Remaster)")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Title", Rule: "required", Value: ""}}}
		h := apiconnect.NewMusicReleaseHandler(svc, nil)

		_, err := h.CreateMusicRelease(context.Background(), connect.NewRequest(&musicv1.CreateMusicReleaseRequest{MusicRelease: validProtoMusicRelease()}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateMusicRelease with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestMusicReleaseHandler_GetMusicRelease(t *testing.T) {
	svc := newFakeMusicReleaseService()
	h := apiconnect.NewMusicReleaseHandler(svc, nil)
	svc.byID["r1"] = &music.Release{ID: "r1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Existing", Status: music.ReleaseStatusStub}

	res, err := h.GetMusicRelease(context.Background(), connect.NewRequest(&musicv1.GetMusicReleaseRequest{Id: "r1"}))
	if err != nil {
		t.Fatalf("GetMusicRelease returned error: %v", err)
	}
	if res.Msg.GetMusicRelease().GetTitle() != "Existing" {
		t.Fatalf("GetMusicRelease returned Title %q, want %q", res.Msg.GetMusicRelease().GetTitle(), "Existing")
	}

	_, err = h.GetMusicRelease(context.Background(), connect.NewRequest(&musicv1.GetMusicReleaseRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetMusicRelease on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}
