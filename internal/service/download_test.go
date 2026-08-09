package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeDownloadClient is a minimal ports.DownloadClient double — see
// fakeIndexerSearcher's own doc comment for the convention this follows.
type fakeDownloadClient struct {
	protocol ports.Protocol

	addID  string
	addErr error
	gotAdd ports.AddDownloadRequest

	status      ports.DownloadStatus
	statusErr   error
	gotStatusID string

	removeErr      error
	gotRemoveID    string
	gotDeleteFiles bool
}

var _ ports.DownloadClient = (*fakeDownloadClient)(nil)

func (f *fakeDownloadClient) Protocol() ports.Protocol { return f.protocol }

func (f *fakeDownloadClient) Add(_ context.Context, req ports.AddDownloadRequest) (string, error) {
	f.gotAdd = req
	if f.addErr != nil {
		return "", f.addErr
	}
	return f.addID, nil
}

func (f *fakeDownloadClient) Status(_ context.Context, externalID string) (ports.DownloadStatus, error) {
	f.gotStatusID = externalID
	if f.statusErr != nil {
		return ports.DownloadStatus{}, f.statusErr
	}
	return f.status, nil
}

func (f *fakeDownloadClient) Remove(_ context.Context, externalID string, deleteFiles bool) error {
	f.gotRemoveID = externalID
	f.gotDeleteFiles = deleteFiles
	return f.removeErr
}

func TestDownload_SubmitDownload_RoutesToRegisteredProtocol(t *testing.T) {
	torrent := &fakeDownloadClient{protocol: ports.ProtocolTorrent, addID: "hash-1"}
	usenet := &fakeDownloadClient{protocol: ports.ProtocolUsenet, addID: "nzo-1"}
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{
		ports.ProtocolTorrent: torrent,
		ports.ProtocolUsenet:  usenet,
	})

	req := ports.AddDownloadRequest{Protocol: ports.ProtocolTorrent, DownloadURL: "magnet:?xt=urn:btih:abc", Title: "Some Release", Category: "movies"}
	id, err := s.SubmitDownload(context.Background(), req)
	if err != nil {
		t.Fatalf("SubmitDownload returned error: %v", err)
	}
	if id != "hash-1" {
		t.Errorf("SubmitDownload id = %q, want %q", id, "hash-1")
	}
	if torrent.gotAdd != req {
		t.Errorf("torrent client got Add(%+v), want %+v", torrent.gotAdd, req)
	}
	if usenet.gotAdd != (ports.AddDownloadRequest{}) {
		t.Errorf("usenet client should not have been called, got Add(%+v)", usenet.gotAdd)
	}
}

func TestDownload_SubmitDownload_UnregisteredProtocol(t *testing.T) {
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{})

	_, err := s.SubmitDownload(context.Background(), ports.AddDownloadRequest{Protocol: ports.ProtocolTorrent})
	if !errors.Is(err, service.ErrUnsupportedProtocol) {
		t.Fatalf("SubmitDownload error = %v, want wrapping ErrUnsupportedProtocol", err)
	}
}

func TestDownload_SubmitDownload_PropagatesClientError(t *testing.T) {
	wantErr := errors.New("qbittorrent unavailable")
	torrent := &fakeDownloadClient{protocol: ports.ProtocolTorrent, addErr: wantErr}
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{ports.ProtocolTorrent: torrent})

	_, err := s.SubmitDownload(context.Background(), ports.AddDownloadRequest{Protocol: ports.ProtocolTorrent})
	if !errors.Is(err, wantErr) {
		t.Fatalf("SubmitDownload error = %v, want %v", err, wantErr)
	}
}

func TestDownload_GetDownloadStatus_RoutesToRegisteredProtocol(t *testing.T) {
	want := ports.DownloadStatus{ExternalID: "hash-1", State: ports.DownloadStateDownloading, Progress: 0.5}
	torrent := &fakeDownloadClient{protocol: ports.ProtocolTorrent, status: want}
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{ports.ProtocolTorrent: torrent})

	got, err := s.GetDownloadStatus(context.Background(), ports.ProtocolTorrent, "hash-1")
	if err != nil {
		t.Fatalf("GetDownloadStatus returned error: %v", err)
	}
	if got != want {
		t.Errorf("GetDownloadStatus = %+v, want %+v", got, want)
	}
	if torrent.gotStatusID != "hash-1" {
		t.Errorf("torrent client got Status(%q), want %q", torrent.gotStatusID, "hash-1")
	}
}

func TestDownload_GetDownloadStatus_UnregisteredProtocol(t *testing.T) {
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{})

	_, err := s.GetDownloadStatus(context.Background(), ports.ProtocolUsenet, "nzo-1")
	if !errors.Is(err, service.ErrUnsupportedProtocol) {
		t.Fatalf("GetDownloadStatus error = %v, want wrapping ErrUnsupportedProtocol", err)
	}
}

func TestDownload_GetDownloadStatus_PropagatesNotFound(t *testing.T) {
	torrent := &fakeDownloadClient{protocol: ports.ProtocolTorrent, statusErr: ports.ErrNotFound}
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{ports.ProtocolTorrent: torrent})

	_, err := s.GetDownloadStatus(context.Background(), ports.ProtocolTorrent, "unknown")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDownloadStatus error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func TestDownload_RemoveDownload_RoutesToRegisteredProtocol(t *testing.T) {
	torrent := &fakeDownloadClient{protocol: ports.ProtocolTorrent}
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{ports.ProtocolTorrent: torrent})

	if err := s.RemoveDownload(context.Background(), ports.ProtocolTorrent, "hash-1", true); err != nil {
		t.Fatalf("RemoveDownload returned error: %v", err)
	}
	if torrent.gotRemoveID != "hash-1" || !torrent.gotDeleteFiles {
		t.Errorf("torrent client got Remove(%q, %v), want (%q, true)", torrent.gotRemoveID, torrent.gotDeleteFiles, "hash-1")
	}
}

func TestDownload_RemoveDownload_UnregisteredProtocol(t *testing.T) {
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{})

	err := s.RemoveDownload(context.Background(), ports.ProtocolTorrent, "hash-1", false)
	if !errors.Is(err, service.ErrUnsupportedProtocol) {
		t.Fatalf("RemoveDownload error = %v, want wrapping ErrUnsupportedProtocol", err)
	}
}

func TestDownload_RemoveDownload_PropagatesNotFound(t *testing.T) {
	torrent := &fakeDownloadClient{protocol: ports.ProtocolTorrent, removeErr: ports.ErrNotFound}
	s := service.NewDownload(map[ports.Protocol]ports.DownloadClient{ports.ProtocolTorrent: torrent})

	err := s.RemoveDownload(context.Background(), ports.ProtocolTorrent, "unknown", false)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("RemoveDownload error = %v, want wrapping ports.ErrNotFound", err)
	}
}
