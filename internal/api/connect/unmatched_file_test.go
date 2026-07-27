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

	listGroupFiles []*domain.UnmatchedFile
	listGroupErr   error
	listGroupKey   string

	dismissBatchFiles []*domain.UnmatchedFile
	dismissBatchErr   error
	dismissBatchIDs   []string

	acceptCandidateErr         error
	acceptCandidateGroupKey    string
	acceptCandidateExternalRef string
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

func (f *fakeUnmatchedFileService) ListGroup(_ context.Context, groupKey string) ([]*domain.UnmatchedFile, error) {
	f.listGroupKey = groupKey
	if f.listGroupErr != nil {
		return nil, f.listGroupErr
	}
	return f.listGroupFiles, nil
}

func (f *fakeUnmatchedFileService) DismissBatch(_ context.Context, ids []string) ([]*domain.UnmatchedFile, error) {
	f.dismissBatchIDs = ids
	if f.dismissBatchErr != nil {
		return nil, f.dismissBatchErr
	}
	return f.dismissBatchFiles, nil
}

func (f *fakeUnmatchedFileService) AcceptCandidate(_ context.Context, groupKey, externalRef string) error {
	f.acceptCandidateGroupKey = groupKey
	f.acceptCandidateExternalRef = externalRef
	return f.acceptCandidateErr
}

func TestUnmatchedFileHandler_GetUnmatchedFile(t *testing.T) {
	now := time.Now()
	file := &domain.UnmatchedFile{
		ID:           "uf-1",
		Path:         "/media/incoming/uf-1.flac",
		GroupKey:     "/media/incoming/album",
		DiscNumber:   1,
		TrackNumber:  "A1",
		Size:         123456,
		OSHash:       "0123456789abcdef",
		SHA1:         "a9993e364706816aba3e25717850c26c9cd0d89d",
		DiscoveredAt: now,
		Status:       domain.UnmatchedFileStatusPending,
		Fingerprint:  &domain.Fingerprint{Tags: map[string]string{"artist": "Test Artist"}},
		Candidates: []domain.MatchCandidate{
			{ExternalRef: "mbid-1", Title: "Test Release", Score: 0.9, Tier: domain.MatchTierDirectID, Signals: map[string]float64{"acoustid": 0.9}},
		},
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
	if got.GetGroupKey() != file.GroupKey || got.GetDiscNumber() != 1 || got.GetTrackNumber() != "A1" {
		t.Fatalf("GetUnmatchedFile did not round-trip GroupKey/DiscNumber/TrackNumber: %+v", got)
	}
	if got.GetFingerprint().GetTags()["artist"] != "Test Artist" {
		t.Fatalf("GetUnmatchedFile did not round-trip Fingerprint.Tags: %+v", got.GetFingerprint())
	}
	if len(got.GetCandidates()) != 1 || got.GetCandidates()[0].GetExternalRef() != "mbid-1" ||
		got.GetCandidates()[0].GetTier() != pipelinev1.MatchTier_MATCH_TIER_DIRECT_ID {
		t.Fatalf("GetUnmatchedFile did not round-trip Candidates: %+v", got.GetCandidates())
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

func TestUnmatchedFileHandler_ListGroupUnmatchedFiles(t *testing.T) {
	files := []*domain.UnmatchedFile{
		{ID: "uf-1", GroupKey: "album-1"},
		{ID: "uf-2", GroupKey: "album-1"},
	}
	svc := &fakeUnmatchedFileService{listGroupFiles: files}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	resp, err := h.ListGroupUnmatchedFiles(context.Background(), connect.NewRequest(&pipelinev1.ListGroupUnmatchedFilesRequest{GroupKey: "album-1"}))
	if err != nil {
		t.Fatalf("ListGroupUnmatchedFiles returned error: %v", err)
	}
	if svc.listGroupKey != "album-1" {
		t.Fatalf("ListGroupUnmatchedFiles passed group_key %q, want %q", svc.listGroupKey, "album-1")
	}
	if got := resp.Msg.GetUnmatchedFiles(); len(got) != 2 || got[0].GetId() != "uf-1" || got[1].GetId() != "uf-2" {
		t.Fatalf("ListGroupUnmatchedFiles returned unexpected files: %+v", got)
	}
}

func TestUnmatchedFileHandler_ListGroupUnmatchedFiles_Error(t *testing.T) {
	svc := &fakeUnmatchedFileService{listGroupErr: ports.ErrNotFound}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.ListGroupUnmatchedFiles(context.Background(), connect.NewRequest(&pipelinev1.ListGroupUnmatchedFilesRequest{GroupKey: "album-1"}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("ListGroupUnmatchedFiles returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("ListGroupUnmatchedFiles returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}

func TestUnmatchedFileHandler_DismissUnmatchedFileBatch(t *testing.T) {
	files := []*domain.UnmatchedFile{
		{ID: "uf-1", Status: domain.UnmatchedFileStatusDismissed},
		{ID: "uf-2", Status: domain.UnmatchedFileStatusDismissed},
	}
	svc := &fakeUnmatchedFileService{dismissBatchFiles: files}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	resp, err := h.DismissUnmatchedFileBatch(context.Background(), connect.NewRequest(&pipelinev1.DismissUnmatchedFileBatchRequest{
		UnmatchedFileIds: []string{"uf-1", "uf-2"},
	}))
	if err != nil {
		t.Fatalf("DismissUnmatchedFileBatch returned error: %v", err)
	}
	if len(svc.dismissBatchIDs) != 2 || svc.dismissBatchIDs[0] != "uf-1" || svc.dismissBatchIDs[1] != "uf-2" {
		t.Fatalf("DismissUnmatchedFileBatch passed ids %v, want [uf-1 uf-2]", svc.dismissBatchIDs)
	}
	got := resp.Msg.GetUnmatchedFiles()
	if len(got) != 2 || got[0].GetStatus() != pipelinev1.UnmatchedFileStatus_UNMATCHED_FILE_STATUS_DISMISSED {
		t.Fatalf("DismissUnmatchedFileBatch returned unexpected files: %+v", got)
	}
}

func TestUnmatchedFileHandler_DismissUnmatchedFileBatch_Error(t *testing.T) {
	svc := &fakeUnmatchedFileService{dismissBatchErr: ports.ErrNotFound}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.DismissUnmatchedFileBatch(context.Background(), connect.NewRequest(&pipelinev1.DismissUnmatchedFileBatchRequest{
		UnmatchedFileIds: []string{"missing"},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("DismissUnmatchedFileBatch returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("DismissUnmatchedFileBatch returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}

func TestUnmatchedFileHandler_AcceptCandidate(t *testing.T) {
	svc := &fakeUnmatchedFileService{}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.AcceptCandidate(context.Background(), connect.NewRequest(&pipelinev1.AcceptCandidateRequest{
		GroupKey:    "group-1",
		ExternalRef: "release-mbid-1",
	}))
	if err != nil {
		t.Fatalf("AcceptCandidate returned error: %v", err)
	}
	if svc.acceptCandidateGroupKey != "group-1" || svc.acceptCandidateExternalRef != "release-mbid-1" {
		t.Fatalf("AcceptCandidate passed (groupKey, externalRef) = (%q, %q), want (group-1, release-mbid-1)", svc.acceptCandidateGroupKey, svc.acceptCandidateExternalRef)
	}
}

func TestUnmatchedFileHandler_AcceptCandidate_Error(t *testing.T) {
	svc := &fakeUnmatchedFileService{acceptCandidateErr: ports.ErrNotFound}
	h := apiconnect.NewUnmatchedFileHandler(svc, nil)

	_, err := h.AcceptCandidate(context.Background(), connect.NewRequest(&pipelinev1.AcceptCandidateRequest{
		GroupKey:    "missing-group",
		ExternalRef: "release-mbid-1",
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("AcceptCandidate returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("AcceptCandidate returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}
