package apiconnect

import (
	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
	"purser/internal/ports"

	"google.golang.org/protobuf/types/known/durationpb"
)

// protoToProtocol maps the wire enum to ports.Protocol — the reverse of
// protocolToProto (indexer_search_convert.go). PROTOCOL_UNSPECIFIED and any
// unrecognized value map to the zero ports.Protocol ("") rather than
// panicking; DownloadService's registry lookup then reports that as
// service.ErrUnsupportedProtocol, same as a genuinely unconfigured
// protocol.
func protoToProtocol(p acquisitionv1.Protocol) ports.Protocol {
	switch p {
	case acquisitionv1.Protocol_PROTOCOL_TORRENT:
		return ports.ProtocolTorrent
	case acquisitionv1.Protocol_PROTOCOL_USENET:
		return ports.ProtocolUsenet
	default:
		return ""
	}
}

// downloadStateToProto maps ports.DownloadState to its wire enum. An
// unrecognized value (never expected from a well-behaved DownloadClient
// adapter) maps to DOWNLOAD_STATE_UNSPECIFIED rather than panicking.
func downloadStateToProto(s ports.DownloadState) acquisitionv1.DownloadState {
	switch s {
	case ports.DownloadStateQueued:
		return acquisitionv1.DownloadState_DOWNLOAD_STATE_QUEUED
	case ports.DownloadStateDownloading:
		return acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING
	case ports.DownloadStatePaused:
		return acquisitionv1.DownloadState_DOWNLOAD_STATE_PAUSED
	case ports.DownloadStateCompleted:
		return acquisitionv1.DownloadState_DOWNLOAD_STATE_COMPLETED
	case ports.DownloadStateFailed:
		return acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED
	default:
		return acquisitionv1.DownloadState_DOWNLOAD_STATE_UNSPECIFIED
	}
}

// downloadStatusToProto maps ports.DownloadStatus to its wire shape. eta
// is left unset (nil) when s.ETA is nil — a message-typed field is
// nil-able for free in proto3, matching download.proto's own doc comment.
func downloadStatusToProto(s ports.DownloadStatus) *acquisitionv1.DownloadStatus {
	out := &acquisitionv1.DownloadStatus{
		ExternalId: s.ExternalID,
		State:      downloadStateToProto(s.State),
		Progress:   s.Progress,
		SavePath:   s.SavePath,
	}
	if s.ETA != nil {
		out.Eta = durationpb.New(*s.ETA)
	}
	return out
}
