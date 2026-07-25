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

func (f *fakeUnmatchedFileRepository) Update(_ context.Context, _ *domain.UnmatchedFile) error {
	return nil
}

// GetByHash is unused by UnmatchedFileService's tests — present solely to
// satisfy ports.UnmatchedFileRepository.
func (f *fakeUnmatchedFileRepository) GetByHash(context.Context, string, string, string, string) (*domain.UnmatchedFile, error) {
	return nil, ports.ErrNotFound
}

func TestUnmatchedFileService_Get(t *testing.T) {
	file := &domain.UnmatchedFile{ID: "uf-1", Status: domain.UnmatchedFileStatusPending}
	svc := service.NewUnmatchedFileService(&fakeUnmatchedFileRepository{getFile: file})

	got, err := svc.Get(context.Background(), "uf-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != file {
		t.Fatal("Get did not return the repository's file")
	}
}

func TestUnmatchedFileService_Get_NotFound(t *testing.T) {
	svc := service.NewUnmatchedFileService(&fakeUnmatchedFileRepository{getErr: ports.ErrNotFound})

	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get returned %v, want ports.ErrNotFound", err)
	}
}

func TestUnmatchedFileService_List(t *testing.T) {
	files := []*domain.UnmatchedFile{{ID: "uf-1"}, {ID: "uf-2"}}
	repo := &fakeUnmatchedFileRepository{listFiles: files, listNextPageToken: "uf-2"}
	svc := service.NewUnmatchedFileService(repo)

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
	svc := service.NewUnmatchedFileService(&fakeUnmatchedFileRepository{listErr: wantErr})

	if _, _, err := svc.List(context.Background(), "", 0, ""); !errors.Is(err, wantErr) {
		t.Fatalf("List returned %v, want %v", err, wantErr)
	}
}
