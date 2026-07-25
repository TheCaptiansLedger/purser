package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"

	"connectrpc.com/connect"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"

	apiconnect "purser/internal/api/connect"
)

type fakeUnmatchedFileService struct {
	getFile *domain.UnmatchedFile
	getErr  error

	listFiles         []*domain.UnmatchedFile
	listNextPageToken string
	listErr           error
	listStatus        domain.UnmatchedFileStatus
	listPageSize      int
	listPageToken     string

	resolveMediaFile *domain.MediaFile
	resolveFile      *domain.UnmatchedFile
	resolveErr       error
	resolveID        string
	resolveItemID    string
	resolveDismiss   bool
}

func (f *fakeUnmatchedFileService) Get(_ context.Context, _ string) (*domain.UnmatchedFile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getFile, nil
}

func (f *fakeUnmatchedFileService) List(_ context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error) {
	f.listStatus = status
	f.listPageSize = pageSize
	f.listPageToken = pageToken
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listFiles, f.listNextPageToken, nil
}

func (f *fakeUnmatchedFileService) Resolve(_ context.Context, id, itemID string, dismiss bool) (*domain.MediaFile, *domain.UnmatchedFile, error) {
	f.resolveID = id
	f.resolveItemID = itemID
	f.resolveDismiss = dismiss
	if f.resolveErr != nil {
		return nil, nil, f.resolveErr
	}
	return f.resolveMediaFile, f.resolveFile, nil
}

func TestUnmatchedFileHandler_GetUnmatchedFile(t *testing.T) {
	now := time.Now()
	file := &domain.UnmatchedFile{
		ID:           "uf-1",
		Path:         "/media/incoming/uf-1.flac",
		Size:         123456,
		OSHash:       "0123456789abcdef",
		SHA1:         "a9993e364706816aba3e25717850c26c9cd0d89d",
		DiscoveredAt: now,
		Status:       domain.UnmatchedFileStatusPending,
	}
	svc := &fakeUnmatchedFileService{getFile: file}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	resp, err := h.GetUnmatchedFile(context.Background(), connect.NewRequest(&pipelinev1.GetUnmatchedFileRequest{Id: "uf-1"}))
	if err != nil {
		t.Fatalf("GetUnmatchedFile returned error: %v", err)
	}
	got := resp.Msg.GetUnmatchedFile()
	if got.GetId() != "uf-1" || got.GetPath() != file.Path {
		t.Fatalf("GetUnmatchedFile returned unexpected file: %+v", got)
	}
	if got.GetOsHash() != file.OSHash || got.GetSha1() != file.SHA1 {
		t.Fatalf("GetUnmatchedFile did not round-trip hashes: %+v", got)
	}
	if got.GetStatus() != pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_PENDING {
		t.Fatalf("GetUnmatchedFile returned status %v, want PENDING", got.GetStatus())
	}
}

func TestUnmatchedFileHandler_GetUnmatchedFile_NotFound(t *testing.T) {
	svc := &fakeUnmatchedFileService{getErr: ports.ErrNotFound}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.GetUnmatchedFile(context.Background(), connect.NewRequest(&pipelinev1.GetUnmatchedFileRequest{Id: "missing"}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("GetUnmatchedFile returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("GetUnmatchedFile returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}

func TestUnmatchedFileHandler_ListUnmatchedFiles(t *testing.T) {
	files := []*domain.UnmatchedFile{
		{ID: "uf-1", Status: domain.UnmatchedFileStatusPending},
	}
	svc := &fakeUnmatchedFileService{listFiles: files, listNextPageToken: "uf-1"}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	resp, err := h.ListUnmatchedFiles(context.Background(), connect.NewRequest(&pipelinev1.ListUnmatchedFilesRequest{
		PageSize:  2,
		PageToken: "prev-token",
		Status:    pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_PENDING,
	}))
	if err != nil {
		t.Fatalf("ListUnmatchedFiles returned error: %v", err)
	}
	if len(resp.Msg.GetUnmatchedFiles()) != 1 || resp.Msg.GetUnmatchedFiles()[0].GetId() != "uf-1" {
		t.Fatalf("ListUnmatchedFiles returned unexpected files: %+v", resp.Msg.GetUnmatchedFiles())
	}
	if resp.Msg.GetNextPageToken() != "uf-1" {
		t.Fatalf("ListUnmatchedFiles returned next_page_token %q, want %q", resp.Msg.GetNextPageToken(), "uf-1")
	}
	if svc.listStatus != domain.UnmatchedFileStatusPending || svc.listPageSize != 2 || svc.listPageToken != "prev-token" {
		t.Fatalf("ListUnmatchedFiles passed (status=%q, pageSize=%d, pageToken=%q), want (pending, 2, prev-token)",
			svc.listStatus, svc.listPageSize, svc.listPageToken)
	}
}

func TestUnmatchedFileHandler_ListUnmatchedFiles_NoFilter(t *testing.T) {
	svc := &fakeUnmatchedFileService{}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.ListUnmatchedFiles(context.Background(), connect.NewRequest(&pipelinev1.ListUnmatchedFilesRequest{}))
	if err != nil {
		t.Fatalf("ListUnmatchedFiles returned error: %v", err)
	}
	if svc.listStatus != "" {
		t.Fatalf("ListUnmatchedFiles passed status=%q for an unset request, want empty filter", svc.listStatus)
	}
}

func TestUnmatchedFileHandler_ListUnmatchedFiles_Error(t *testing.T) {
	svc := &fakeUnmatchedFileService{listErr: ports.ErrNotFound}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.ListUnmatchedFiles(context.Background(), connect.NewRequest(&pipelinev1.ListUnmatchedFilesRequest{}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("ListUnmatchedFiles returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("ListUnmatchedFiles returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}

func TestUnmatchedFileHandler_ResolveUnmatchedFile_Match(t *testing.T) {
	mf := &domain.MediaFile{ID: "mf-1", ItemID: "item-1", Path: "/media/incoming/uf-1.flac"}
	svc := &fakeUnmatchedFileService{resolveMediaFile: mf}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	resp, err := h.ResolveUnmatchedFile(context.Background(), connect.NewRequest(&pipelinev1.ResolveUnmatchedFileRequest{
		UnmatchedFileId: "uf-1",
		Outcome:         &pipelinev1.ResolveUnmatchedFileRequest_ItemId{ItemId: "item-1"},
	}))
	if err != nil {
		t.Fatalf("ResolveUnmatchedFile returned error: %v", err)
	}
	if svc.resolveID != "uf-1" || svc.resolveItemID != "item-1" || svc.resolveDismiss {
		t.Fatalf("ResolveUnmatchedFile passed (id=%q, itemID=%q, dismiss=%v), want (uf-1, item-1, false)",
			svc.resolveID, svc.resolveItemID, svc.resolveDismiss)
	}
	got := resp.Msg.GetMediaFile()
	if got == nil || got.GetId() != "mf-1" || got.GetItemId() != "item-1" {
		t.Fatalf("ResolveUnmatchedFile returned MediaFile %+v, want the resolved MediaFile", got)
	}
	if resp.Msg.GetUnmatchedFile() != nil {
		t.Fatalf("ResolveUnmatchedFile(match) also returned an UnmatchedFile: %+v", resp.Msg.GetUnmatchedFile())
	}
}

func TestUnmatchedFileHandler_ResolveUnmatchedFile_Dismiss(t *testing.T) {
	uf := &domain.UnmatchedFile{ID: "uf-1", Status: domain.UnmatchedFileStatusDismissed}
	svc := &fakeUnmatchedFileService{resolveFile: uf}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	resp, err := h.ResolveUnmatchedFile(context.Background(), connect.NewRequest(&pipelinev1.ResolveUnmatchedFileRequest{
		UnmatchedFileId: "uf-1",
		Outcome:         &pipelinev1.ResolveUnmatchedFileRequest_Dismiss{Dismiss: true},
	}))
	if err != nil {
		t.Fatalf("ResolveUnmatchedFile returned error: %v", err)
	}
	if svc.resolveID != "uf-1" || !svc.resolveDismiss || svc.resolveItemID != "" {
		t.Fatalf("ResolveUnmatchedFile passed (id=%q, itemID=%q, dismiss=%v), want (uf-1, \"\", true)",
			svc.resolveID, svc.resolveItemID, svc.resolveDismiss)
	}
	got := resp.Msg.GetUnmatchedFile()
	if got == nil || got.GetId() != "uf-1" || got.GetStatus() != pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_DISMISSED {
		t.Fatalf("ResolveUnmatchedFile(dismiss) returned %+v, want status=dismissed", got)
	}
	if resp.Msg.GetMediaFile() != nil {
		t.Fatalf("ResolveUnmatchedFile(dismiss) also returned a MediaFile: %+v", resp.Msg.GetMediaFile())
	}
}

func TestUnmatchedFileHandler_ResolveUnmatchedFile_ItemNotFound(t *testing.T) {
	svc := &fakeUnmatchedFileService{resolveErr: ports.ErrNotFound}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.ResolveUnmatchedFile(context.Background(), connect.NewRequest(&pipelinev1.ResolveUnmatchedFileRequest{
		UnmatchedFileId: "uf-1",
		Outcome:         &pipelinev1.ResolveUnmatchedFileRequest_ItemId{ItemId: "missing-item"},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("ResolveUnmatchedFile returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("ResolveUnmatchedFile returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}
