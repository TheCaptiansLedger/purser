package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	musicv1 "purser/gen/go/purser/music/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeMusicReleaseService struct {
	byID          map[string]*music.Release
	createErr     error
	getErr        error
	getByMBIDErr  error
	getByBcodeErr error
	updateErr     error
	deleteErr     error
	listErr       error
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

func (f *fakeMusicReleaseService) GetByMBID(_ context.Context, mbid string) (*music.Release, error) {
	if f.getByMBIDErr != nil {
		return nil, f.getByMBIDErr
	}
	for _, r := range f.byID {
		if r.MBID == mbid {
			return r, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *fakeMusicReleaseService) GetByBarcode(_ context.Context, barcode string) (*music.Release, error) {
	if f.getByBcodeErr != nil {
		return nil, f.getByBcodeErr
	}
	for _, r := range f.byID {
		if r.Barcode == barcode {
			return r, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *fakeMusicReleaseService) Update(_ context.Context, r *music.Release) (*music.Release, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[r.ID] = r
	return r, nil
}

func (f *fakeMusicReleaseService) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeMusicReleaseService) List(_ context.Context, _ int, _ string) ([]*music.Release, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	releases := make([]*music.Release, 0, len(f.byID))
	for _, r := range f.byID {
		releases = append(releases, r)
	}
	return releases, "", nil
}

func (f *fakeMusicReleaseService) ListByGroup(_ context.Context, groupID string, _ int, _ string) ([]*music.Release, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var releases []*music.Release
	for _, r := range f.byID {
		if r.GroupID == groupID {
			releases = append(releases, r)
		}
	}
	return releases, "", nil
}

func (f *fakeMusicReleaseService) ListByEntry(_ context.Context, libraryEntryID string, _ int, _ string) ([]*music.Release, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var releases []*music.Release
	for _, r := range f.byID {
		if r.LibraryEntryID == libraryEntryID {
			releases = append(releases, r)
		}
	}
	return releases, "", nil
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

func TestMusicReleaseHandler_GetMusicReleaseByMBID(t *testing.T) {
	svc := newFakeMusicReleaseService()
	h := apiconnect.NewMusicReleaseHandler(svc, nil)
	svc.byID["r1"] = &music.Release{ID: "r1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Existing", Status: music.ReleaseStatusStub, MBID: "mbid-1"}

	res, err := h.GetMusicReleaseByMBID(context.Background(), connect.NewRequest(&musicv1.GetMusicReleaseByMBIDRequest{Mbid: "mbid-1"}))
	if err != nil {
		t.Fatalf("GetMusicReleaseByMBID returned error: %v", err)
	}
	if res.Msg.GetMusicRelease().GetId() != "r1" {
		t.Fatalf("GetMusicReleaseByMBID returned Id %q, want %q", res.Msg.GetMusicRelease().GetId(), "r1")
	}

	_, err = h.GetMusicReleaseByMBID(context.Background(), connect.NewRequest(&musicv1.GetMusicReleaseByMBIDRequest{Mbid: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetMusicReleaseByMBID on unknown MBID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestMusicReleaseHandler_GetMusicReleaseByBarcode(t *testing.T) {
	svc := newFakeMusicReleaseService()
	h := apiconnect.NewMusicReleaseHandler(svc, nil)
	svc.byID["r1"] = &music.Release{ID: "r1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Existing", Status: music.ReleaseStatusStub, Barcode: "barcode-1"}

	res, err := h.GetMusicReleaseByBarcode(context.Background(), connect.NewRequest(&musicv1.GetMusicReleaseByBarcodeRequest{Barcode: "barcode-1"}))
	if err != nil {
		t.Fatalf("GetMusicReleaseByBarcode returned error: %v", err)
	}
	if res.Msg.GetMusicRelease().GetId() != "r1" {
		t.Fatalf("GetMusicReleaseByBarcode returned Id %q, want %q", res.Msg.GetMusicRelease().GetId(), "r1")
	}

	_, err = h.GetMusicReleaseByBarcode(context.Background(), connect.NewRequest(&musicv1.GetMusicReleaseByBarcodeRequest{Barcode: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetMusicReleaseByBarcode on unknown barcode returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestMusicReleaseHandler_UpdateMusicRelease(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)
		svc.byID["r1"] = &music.Release{ID: "r1", GroupID: "group1", LibraryEntryID: "entry1", Title: "Original", Country: "Original Country", Status: music.ReleaseStatusStub}

		req := &musicv1.UpdateMusicReleaseRequest{
			MusicRelease: &musicv1.Release{Id: "r1", Title: "New Title", Country: "Should be ignored"},
			UpdateMask:   &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		}
		res, err := h.UpdateMusicRelease(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateMusicRelease returned error: %v", err)
		}
		if res.Msg.GetMusicRelease().GetTitle() != "New Title" {
			t.Fatalf("UpdateMusicRelease applied Title %q, want %q", res.Msg.GetMusicRelease().GetTitle(), "New Title")
		}
		if res.Msg.GetMusicRelease().GetCountry() != "Original Country" {
			t.Fatalf("UpdateMusicRelease touched Country: got %q", res.Msg.GetMusicRelease().GetCountry())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)

		_, err := h.UpdateMusicRelease(context.Background(), connect.NewRequest(&musicv1.UpdateMusicReleaseRequest{MusicRelease: &musicv1.Release{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateMusicRelease on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestMusicReleaseHandler_DeleteMusicRelease(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)
		svc.byID["r1"] = &music.Release{ID: "r1"}

		if _, err := h.DeleteMusicRelease(context.Background(), connect.NewRequest(&musicv1.DeleteMusicReleaseRequest{Id: "r1"})); err != nil {
			t.Fatalf("DeleteMusicRelease returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewMusicReleaseHandler(svc, nil)

		_, err := h.DeleteMusicRelease(context.Background(), connect.NewRequest(&musicv1.DeleteMusicReleaseRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteMusicRelease on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestMusicReleaseHandler_ListMusicReleases(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)
		svc.byID["r1"] = &music.Release{ID: "r1"}
		svc.byID["r2"] = &music.Release{ID: "r2"}

		res, err := h.ListMusicReleases(context.Background(), connect.NewRequest(&musicv1.ListMusicReleasesRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListMusicReleases returned error: %v", err)
		}
		if len(res.Msg.GetMusicReleases()) != 2 {
			t.Fatalf("ListMusicReleases returned %d releases, want 2", len(res.Msg.GetMusicReleases()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewMusicReleaseHandler(svc, nil)

		_, err := h.ListMusicReleases(context.Background(), connect.NewRequest(&musicv1.ListMusicReleasesRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListMusicReleases with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("group_id filter dispatches to ListByGroup", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)
		svc.byID["r1"] = &music.Release{ID: "r1", GroupID: "groupA"}
		svc.byID["r2"] = &music.Release{ID: "r2", GroupID: "groupB"}

		res, err := h.ListMusicReleases(context.Background(), connect.NewRequest(&musicv1.ListMusicReleasesRequest{PageSize: 10, GroupId: "groupA"}))
		if err != nil {
			t.Fatalf("ListMusicReleases returned error: %v", err)
		}
		if len(res.Msg.GetMusicReleases()) != 1 || res.Msg.GetMusicReleases()[0].GetId() != "r1" {
			t.Fatalf("ListMusicReleases(group_id=groupA) returned %v, want exactly [r1]", res.Msg.GetMusicReleases())
		}
	})

	t.Run("library_entry_id filter dispatches to ListByEntry", func(t *testing.T) {
		svc := newFakeMusicReleaseService()
		h := apiconnect.NewMusicReleaseHandler(svc, nil)
		svc.byID["r1"] = &music.Release{ID: "r1", LibraryEntryID: "entryA"}
		svc.byID["r2"] = &music.Release{ID: "r2", LibraryEntryID: "entryB"}

		res, err := h.ListMusicReleases(context.Background(), connect.NewRequest(&musicv1.ListMusicReleasesRequest{PageSize: 10, LibraryEntryId: "entryA"}))
		if err != nil {
			t.Fatalf("ListMusicReleases returned error: %v", err)
		}
		if len(res.Msg.GetMusicReleases()) != 1 || res.Msg.GetMusicReleases()[0].GetId() != "r1" {
			t.Fatalf("ListMusicReleases(library_entry_id=entryA) returned %v, want exactly [r1]", res.Msg.GetMusicReleases())
		}
	})
}
