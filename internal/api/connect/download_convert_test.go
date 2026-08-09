package apiconnect

import (
	"purser/internal/ports"
	"testing"
	"time"

	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
)

func TestProtoToProtocol(t *testing.T) {
	tests := []struct {
		in   acquisitionv1.Protocol
		want ports.Protocol
	}{
		{acquisitionv1.Protocol_PROTOCOL_TORRENT, ports.ProtocolTorrent},
		{acquisitionv1.Protocol_PROTOCOL_USENET, ports.ProtocolUsenet},
		{acquisitionv1.Protocol_PROTOCOL_UNSPECIFIED, ports.Protocol("")},
	}
	for _, tt := range tests {
		if got := protoToProtocol(tt.in); got != tt.want {
			t.Errorf("protoToProtocol(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDownloadStateToProto(t *testing.T) {
	tests := []struct {
		in   ports.DownloadState
		want acquisitionv1.DownloadState
	}{
		{ports.DownloadStateQueued, acquisitionv1.DownloadState_DOWNLOAD_STATE_QUEUED},
		{ports.DownloadStateDownloading, acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING},
		{ports.DownloadStatePaused, acquisitionv1.DownloadState_DOWNLOAD_STATE_PAUSED},
		{ports.DownloadStateCompleted, acquisitionv1.DownloadState_DOWNLOAD_STATE_COMPLETED},
		{ports.DownloadStateFailed, acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED},
		{ports.DownloadState("unknown"), acquisitionv1.DownloadState_DOWNLOAD_STATE_UNSPECIFIED},
	}
	for _, tt := range tests {
		if got := downloadStateToProto(tt.in); got != tt.want {
			t.Errorf("downloadStateToProto(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestDownloadStatusToProto(t *testing.T) {
	t.Run("with an ETA", func(t *testing.T) {
		eta := 90 * time.Second
		got := downloadStatusToProto(ports.DownloadStatus{
			ExternalID: "hash-1", State: ports.DownloadStateDownloading, Progress: 0.5, SavePath: "/downloads/x", ETA: &eta,
		})
		if got.GetExternalId() != "hash-1" || got.GetState() != acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING {
			t.Errorf("downloadStatusToProto = %+v, want external_id=hash-1 state=DOWNLOADING", got)
		}
		if got.GetProgress() != 0.5 || got.GetSavePath() != "/downloads/x" {
			t.Errorf("downloadStatusToProto = %+v, want progress=0.5 save_path=/downloads/x", got)
		}
		if got.GetEta() == nil || got.GetEta().AsDuration() != eta {
			t.Errorf("downloadStatusToProto.Eta = %v, want %v", got.GetEta(), eta)
		}
	})

	t.Run("with no ETA", func(t *testing.T) {
		got := downloadStatusToProto(ports.DownloadStatus{ExternalID: "hash-1", State: ports.DownloadStateQueued})
		if got.GetEta() != nil {
			t.Errorf("downloadStatusToProto.Eta = %v, want nil", got.GetEta())
		}
	})
}
