package ports

import (
	"context"
	"time"
)

// Protocol is the transport a release/download moves over — torrent or
// Usenet. Shared between IndexerSearcher (IndexerRelease.Protocol reports
// what an indexer offers) and DownloadClient (Protocol() self-declares
// what a client backend handles), so a release's protocol maps directly
// onto which registered client handles it, no translation layer. See
// docs/technical/acquisition-download-client.md.
type Protocol string

// The two Protocol values a DownloadClient/IndexerRelease can carry.
const (
	ProtocolTorrent Protocol = "torrent"
	ProtocolUsenet  Protocol = "usenet"
)

// DownloadState is a client-normalized download state. Adapters own
// mapping their own backend-specific vocabulary (qBittorrent's
// downloading/stalledDL/pausedDL/... vs. SABnzbd's
// Downloading/Paused/Failed/...) onto these values — that mapping is
// adapter-owned per docs/adr/0001-hexagonal-architecture.md's
// "adapter-specific quirks never leak upward into ports" rule.
type DownloadState string

// The normalized DownloadState values every adapter maps its own
// backend-specific state vocabulary onto.
const (
	DownloadStateQueued      DownloadState = "queued"
	DownloadStateDownloading DownloadState = "downloading"
	DownloadStatePaused      DownloadState = "paused"
	DownloadStateCompleted   DownloadState = "completed"
	DownloadStateFailed      DownloadState = "failed"
)

// DownloadClient is the capability-shaped port for submitting a release to
// a configured download-client backend and managing it afterward. It is
// an ordinary port describing capability, not a
// docs/adr/0027-provider-independence.md-style provider exception: an
// operator configures exactly one client per Protocol, swappable, never
// two competing backends answering the same call — the same shape
// ImageFetcher and Datastore already use in this codebase. Kept to
// exactly these four methods per docs/adr/0002-solid-design-principles.md's
// Interface Segregation rule. See
// docs/technical/acquisition-download-client.md and
// docs/technical/acquisition-pipeline.md.
type DownloadClient interface {
	// Protocol self-declares which protocol this adapter handles. The
	// composition root uses this to build a map[Protocol]DownloadClient
	// routing registry — adding a third client adapter is a new
	// implementation, never an edit to routing logic.
	Protocol() Protocol

	// Add submits a release for download and returns the client's own
	// external ID for later Status/Remove calls.
	Add(ctx context.Context, req AddDownloadRequest) (externalID string, err error)

	// Status reports current progress/state for a previously submitted
	// download. Returns ErrNotFound if externalID is unknown to the
	// client (stale or already removed).
	Status(ctx context.Context, externalID string) (DownloadStatus, error)

	// Remove cancels/deletes a submitted download, optionally deleting
	// any partially-downloaded files. Returns ErrNotFound if externalID
	// is unknown to the client.
	Remove(ctx context.Context, externalID string, deleteFiles bool) error
}

// AddDownloadRequest is the input to DownloadClient.Add.
type AddDownloadRequest struct {
	Protocol    Protocol
	DownloadURL string // magnet, .torrent URL, or .nzb URL depending on Protocol
	Title       string
	Category    string // passed through as-is; the client backend owns any path mapping
}

// DownloadStatus is the result of DownloadClient.Status.
type DownloadStatus struct {
	ExternalID string
	State      DownloadState
	Progress   float64 // 0-1
	SavePath   string
	ETA        *time.Duration
}
