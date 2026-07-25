package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeUnmatchedFileRepository struct {
	getFile *domain.UnmatchedFile
	getErr  error

	listFiles         []*domain.UnmatchedFile
	listNextPageToken string
	listErr           error
	listStatus        domain.UnmatchedFileStatus
	listPageSize      int
	listPageToken     string

	updated   *domain.UnmatchedFile
	updateErr error
	deletedID string
	deleteErr error
}

func (f *fakeUnmatchedFileRepository) Create(_ context.Context, _ *domain.UnmatchedFile) error {
	return nil
}

func (f *fakeUnmatchedFileRepository) Get(_ context.Context, _ string) (*domain.UnmatchedFile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getFile, nil
}

func (f *fakeUnmatchedFileRepository) List(_ context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error) {
	f.listStatus = status
	f.listPageSize = pageSize
	f.listPageToken = pageToken
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listFiles, f.listNextPageToken, nil
}

func (f *fakeUnmatchedFileRepository) Update(_ context.Context, u *domain.UnmatchedFile) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = u
	return nil
}

func (f *fakeUnmatchedFileRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedID = id
	return nil
}

// GetByHash is unused by UnmatchedFileService's tests — present solely to
// satisfy ports.UnmatchedFileRepository.
func (f *fakeUnmatchedFileRepository) GetByHash(context.Context, string, string, string, string) (*domain.UnmatchedFile, error) {
	return nil, ports.ErrNotFound
}

func newUnmatchedFileService(repo *fakeUnmatchedFileRepository, items *fakeItemRepository, mediaFiles *fakeMediaFileRepository) *service.UnmatchedFileService {
	if items == nil {
		items = newFakeItemRepository()
	}
	if mediaFiles == nil {
		mediaFiles = newFakeMediaFileRepository()
	}
	return service.NewUnmatchedFileService(repo, items, mediaFiles)
}

func TestUnmatchedFileService_Get(t *testing.T) {
	file := &domain.UnmatchedFile{ID: "uf-1", Status: domain.UnmatchedFileStatusPending}
	svc := newUnmatchedFileService(&fakeUnmatchedFileRepository{getFile: file}, nil, nil)

	got, err := svc.Get(context.Background(), "uf-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != file {
		t.Fatal("Get did not return the repository's file")
	}
}

func TestUnmatchedFileService_Get_NotFound(t *testing.T) {
	svc := newUnmatchedFileService(&fakeUnmatchedFileRepository{getErr: ports.ErrNotFound}, nil, nil)

	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get returned %v, want ports.ErrNotFound", err)
	}
}

func TestUnmatchedFileService_List(t *testing.T) {
	files := []*domain.UnmatchedFile{{ID: "uf-1"}, {ID: "uf-2"}}
	repo := &fakeUnmatchedFileRepository{listFiles: files, listNextPageToken: "uf-2"}
	svc := newUnmatchedFileService(repo, nil, nil)

	got, next, err := svc.List(context.Background(), domain.UnmatchedFileStatusPending, 2, "token")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 2 || next != "uf-2" {
		t.Fatalf("List returned (%v, %q), want (%v, %q)", got, next, files, "uf-2")
	}
	if repo.listStatus != domain.UnmatchedFileStatusPending || repo.listPageSize != 2 || repo.listPageToken != "token" {
		t.Fatalf("List passed (status=%q, pageSize=%d, pageToken=%q), want (pending, 2, token)",
			repo.listStatus, repo.listPageSize, repo.listPageToken)
	}
}

func TestUnmatchedFileService_List_Error(t *testing.T) {
	wantErr := errors.New("boom")
	svc := newUnmatchedFileService(&fakeUnmatchedFileRepository{listErr: wantErr}, nil, nil)

	if _, _, err := svc.List(context.Background(), "", 0, ""); !errors.Is(err, wantErr) {
		t.Fatalf("List returned %v, want %v", err, wantErr)
	}
}

func TestUnmatchedFileService_Resolve_NotFound(t *testing.T) {
	svc := newUnmatchedFileService(&fakeUnmatchedFileRepository{getErr: ports.ErrNotFound}, nil, nil)

	if _, _, err := svc.Resolve(context.Background(), "missing", "item-1", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Resolve returned %v, want ports.ErrNotFound", err)
	}
}

func TestUnmatchedFileService_Resolve_Dismiss(t *testing.T) {
	file := &domain.UnmatchedFile{ID: "uf-1", Path: "/media/incoming/uf-1.flac", Status: domain.UnmatchedFileStatusPending}
	repo := &fakeUnmatchedFileRepository{getFile: file}
	svc := newUnmatchedFileService(repo, nil, nil)

	mf, uf, err := svc.Resolve(context.Background(), "uf-1", "", true)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if mf != nil {
		t.Fatalf("Resolve(dismiss) returned a MediaFile %+v, want nil", mf)
	}
	if uf == nil || uf.Status != domain.UnmatchedFileStatusDismissed {
		t.Fatalf("Resolve(dismiss) returned %+v, want status=dismissed", uf)
	}
	if repo.updated == nil || repo.updated.Status != domain.UnmatchedFileStatusDismissed {
		t.Fatalf("Resolve(dismiss) did not persist status=dismissed via Update, got %+v", repo.updated)
	}
	if repo.deletedID != "" {
		t.Fatalf("Resolve(dismiss) called Delete(%q), want no Delete call", repo.deletedID)
	}
}

func TestUnmatchedFileService_Resolve_NeitherSet(t *testing.T) {
	file := &domain.UnmatchedFile{ID: "uf-1", Status: domain.UnmatchedFileStatusPending}
	svc := newUnmatchedFileService(&fakeUnmatchedFileRepository{getFile: file}, nil, nil)

	_, _, err := svc.Resolve(context.Background(), "uf-1", "", false)
	var verr *domain.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Resolve(neither set) returned %v, want a *domain.ValidationError", err)
	}
}

func TestUnmatchedFileService_Resolve_Match(t *testing.T) {
	file := &domain.UnmatchedFile{
		ID: "uf-1", Path: "/media/incoming/uf-1.flac", Size: 123,
		OSHash: "oshash-1", SHA1: "sha1-1", Status: domain.UnmatchedFileStatusPending,
	}
	repo := &fakeUnmatchedFileRepository{getFile: file}
	items := newFakeItemRepository()
	items.byID["item-1"] = &domain.Item{ID: "item-1"}
	mediaFiles := newFakeMediaFileRepository()
	svc := newUnmatchedFileService(repo, items, mediaFiles)

	mf, uf, err := svc.Resolve(context.Background(), "uf-1", "item-1", false)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if uf != nil {
		t.Fatalf("Resolve(match) returned an UnmatchedFile %+v, want nil", uf)
	}
	if mf == nil || mf.ItemID != "item-1" || mf.Path != file.Path || mf.OSHash != file.OSHash || mf.SHA1 != file.SHA1 {
		t.Fatalf("Resolve(match) returned %+v, want a MediaFile linking uf-1's path/hashes to item-1", mf)
	}
	if mf.ID == "" {
		t.Fatal("Resolve(match) returned a MediaFile with no server-generated ID")
	}
	if _, ok := mediaFiles.byID[mf.ID]; !ok {
		t.Fatalf("Resolve(match) did not persist the MediaFile via Create, byID=%v", mediaFiles.byID)
	}
	if repo.deletedID != "uf-1" {
		t.Fatalf("Resolve(match) called Delete(%q), want Delete(%q)", repo.deletedID, "uf-1")
	}
}

func TestUnmatchedFileService_Resolve_Match_ItemNotFound(t *testing.T) {
	file := &domain.UnmatchedFile{ID: "uf-1", Status: domain.UnmatchedFileStatusPending}
	repo := &fakeUnmatchedFileRepository{getFile: file}
	svc := newUnmatchedFileService(repo, newFakeItemRepository(), nil)

	if _, _, err := svc.Resolve(context.Background(), "uf-1", "missing-item", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Resolve(match, missing item) returned %v, want ports.ErrNotFound", err)
	}
	if repo.deletedID != "" {
		t.Fatalf("Resolve(match, missing item) called Delete(%q), want no Delete call", repo.deletedID)
	}
}
