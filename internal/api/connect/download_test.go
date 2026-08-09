package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"

	"connectrpc.com/connect"

	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeDownloadService struct {
	gotSubmit ports.AddDownloadRequest
	submitID  string
	submitErr error

	gotStatusProtocol ports.Protocol
	gotStatusID       string
	status            ports.DownloadStatus
	statusErr         error

	gotRemoveProtocol    ports.Protocol
	gotRemoveID          string
	gotRemoveDeleteFiles bool
	removeErr            error
}

func (f *fakeDownloadService) SubmitDownload(_ context.Context, req ports.AddDownloadRequest) (string, error) {
	f.gotSubmit = req
	if f.submitErr != nil {
		return "", f.submitErr
	}
	return f.submitID, nil
}

func (f *fakeDownloadService) GetDownloadStatus(_ context.Context, protocol ports.Protocol, externalID string) (ports.DownloadStatus, error) {
	f.gotStatusProtocol = protocol
	f.gotStatusID = externalID
	if f.statusErr != nil {
		return ports.DownloadStatus{}, f.statusErr
	}
	return f.status, nil
}

func (f *fakeDownloadService) RemoveDownload(_ context.Context, protocol ports.Protocol, externalID string, deleteFiles bool) error {
	f.gotRemoveProtocol = protocol
	f.gotRemoveID = externalID
	f.gotRemoveDeleteFiles = deleteFiles
	return f.removeErr
}

func TestDownloadHandler_SubmitDownload(t *testing.T) {
	t.Run("valid request submits and returns the external id", func(t *testing.T) {
		svc := &fakeDownloadService{submitID: "hash-1"}
		h := apiconnect.NewDownloadHandler(svc, nil)

		res, err := h.SubmitDownload(context.Background(), connect.NewRequest(&acquisitionv1.SubmitDownloadRequest{
			Protocol:    acquisitionv1.Protocol_PROTOCOL_TORRENT,
			DownloadUrl: "magnet:?xt=urn:btih:abc",
			Title:       "Some Release",
			Category:    "movies",
		}))
		if err != nil {
			t.Fatalf("SubmitDownload returned error: %v", err)
		}
		if res.Msg.GetExternalId() != "hash-1" {
			t.Errorf("SubmitDownload external_id = %q, want %q", res.Msg.GetExternalId(), "hash-1")
		}
		if svc.gotSubmit.Protocol != ports.ProtocolTorrent || svc.gotSubmit.DownloadURL != "magnet:?xt=urn:btih:abc" {
			t.Errorf("SubmitDownload passed req=%+v, want protocol=torrent download_url set", svc.gotSubmit)
		}
	})

	t.Run("an unsupported protocol maps to CodeInvalidArgument", func(t *testing.T) {
		svc := &fakeDownloadService{submitErr: service.ErrUnsupportedProtocol}
		h := apiconnect.NewDownloadHandler(svc, nil)

		_, err := h.SubmitDownload(context.Background(), connect.NewRequest(&acquisitionv1.SubmitDownloadRequest{Protocol: acquisitionv1.Protocol_PROTOCOL_TORRENT}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("SubmitDownload with ErrUnsupportedProtocol returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeDownloadService{submitErr: errors.New("boom")}
		h := apiconnect.NewDownloadHandler(svc, nil)

		_, err := h.SubmitDownload(context.Background(), connect.NewRequest(&acquisitionv1.SubmitDownloadRequest{}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("SubmitDownload with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestDownloadHandler_GetDownloadStatus(t *testing.T) {
	t.Run("valid request returns status", func(t *testing.T) {
		svc := &fakeDownloadService{status: ports.DownloadStatus{ExternalID: "hash-1", State: ports.DownloadStateDownloading, Progress: 0.5, SavePath: "/downloads/x"}}
		h := apiconnect.NewDownloadHandler(svc, nil)

		res, err := h.GetDownloadStatus(context.Background(), connect.NewRequest(&acquisitionv1.GetDownloadStatusRequest{
			Protocol:   acquisitionv1.Protocol_PROTOCOL_TORRENT,
			ExternalId: "hash-1",
		}))
		if err != nil {
			t.Fatalf("GetDownloadStatus returned error: %v", err)
		}
		if res.Msg.GetStatus().GetExternalId() != "hash-1" || res.Msg.GetStatus().GetState() != acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING {
			t.Errorf("GetDownloadStatus status = %+v, want external_id=hash-1 state=DOWNLOADING", res.Msg.GetStatus())
		}
		if svc.gotStatusProtocol != ports.ProtocolTorrent || svc.gotStatusID != "hash-1" {
			t.Errorf("GetDownloadStatus passed protocol=%q external_id=%q, want torrent/hash-1", svc.gotStatusProtocol, svc.gotStatusID)
		}
	})

	t.Run("not found maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeDownloadService{statusErr: ports.ErrNotFound}
		h := apiconnect.NewDownloadHandler(svc, nil)

		_, err := h.GetDownloadStatus(context.Background(), connect.NewRequest(&acquisitionv1.GetDownloadStatusRequest{Protocol: acquisitionv1.Protocol_PROTOCOL_TORRENT, ExternalId: "unknown"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetDownloadStatus with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestDownloadHandler_RemoveDownload(t *testing.T) {
	t.Run("valid request removes and returns empty response", func(t *testing.T) {
		svc := &fakeDownloadService{}
		h := apiconnect.NewDownloadHandler(svc, nil)

		_, err := h.RemoveDownload(context.Background(), connect.NewRequest(&acquisitionv1.RemoveDownloadRequest{
			Protocol:    acquisitionv1.Protocol_PROTOCOL_USENET,
			ExternalId:  "nzo-1",
			DeleteFiles: true,
		}))
		if err != nil {
			t.Fatalf("RemoveDownload returned error: %v", err)
		}
		if svc.gotRemoveProtocol != ports.ProtocolUsenet || svc.gotRemoveID != "nzo-1" || !svc.gotRemoveDeleteFiles {
			t.Errorf("RemoveDownload passed protocol=%q external_id=%q delete_files=%v, want usenet/nzo-1/true", svc.gotRemoveProtocol, svc.gotRemoveID, svc.gotRemoveDeleteFiles)
		}
	})

	t.Run("not found maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeDownloadService{removeErr: ports.ErrNotFound}
		h := apiconnect.NewDownloadHandler(svc, nil)

		_, err := h.RemoveDownload(context.Background(), connect.NewRequest(&acquisitionv1.RemoveDownloadRequest{Protocol: acquisitionv1.Protocol_PROTOCOL_TORRENT, ExternalId: "unknown"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("RemoveDownload with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}
