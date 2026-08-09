package service

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/ports"
)

// ErrUnsupportedProtocol is returned when no ports.DownloadClient is
// registered for a requested ports.Protocol — e.g. an operator enabled
// Prowlarr search but never configured qBittorrent/SABnzbd. Mapped to
// connect.CodeInvalidArgument by internal/api/connect's error-mapping
// helper, the same posture jobqueue.ErrUnknownKind already uses for "the
// caller asked for something that was never registered."
var ErrUnsupportedProtocol = errors.New("service: no download client registered for protocol")

// Download exposes internal/adapters/qbittorrent's and
// internal/adapters/sabnzbd's (or any future ports.DownloadClient
// adapter's) submit/status/remove capability directly to a caller. Depends
// only on a map[ports.Protocol]ports.DownloadClient registry — no other
// port — per docs/adr/0011-api-design.md's no-God-service rule: this is a
// one-registry, three-method service, never folded into IndexerSearch or
// vice versa (see that service's own doc comment). The composition root
// builds the registry from every configured adapter's own Protocol(), no
// switch statement — adding a third client later is additive only. See
// proto/purser/acquisition/v1/download.proto's own doc comment and
// docs/technical/acquisition-download-client.md.
type Download struct {
	clients map[ports.Protocol]ports.DownloadClient
}

// NewDownload constructs a Download backed by clients, keyed by each
// adapter's own Protocol(). A protocol absent from clients simply has no
// registered backend — every method below reports that as
// ErrUnsupportedProtocol rather than a nil-map panic.
func NewDownload(clients map[ports.Protocol]ports.DownloadClient) *Download {
	return &Download{clients: clients}
}

func (s *Download) client(protocol ports.Protocol) (ports.DownloadClient, error) {
	client, ok := s.clients[protocol]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProtocol, protocol)
	}
	return client, nil
}

// SubmitDownload routes req to the client registered for req.Protocol — a
// thin passthrough to ports.DownloadClient.Add.
func (s *Download) SubmitDownload(ctx context.Context, req ports.AddDownloadRequest) (string, error) {
	client, err := s.client(req.Protocol)
	if err != nil {
		return "", err
	}
	return client.Add(ctx, req)
}

// GetDownloadStatus routes to the client registered for protocol — a thin
// passthrough to ports.DownloadClient.Status. externalID is only unique
// within the client that issued it, not globally, so protocol is required
// to know which client to ask.
func (s *Download) GetDownloadStatus(ctx context.Context, protocol ports.Protocol, externalID string) (ports.DownloadStatus, error) {
	client, err := s.client(protocol)
	if err != nil {
		return ports.DownloadStatus{}, err
	}
	return client.Status(ctx, externalID)
}

// RemoveDownload routes to the client registered for protocol — a thin
// passthrough to ports.DownloadClient.Remove. See GetDownloadStatus's own
// doc comment for why protocol travels alongside externalID here too.
func (s *Download) RemoveDownload(ctx context.Context, protocol ports.Protocol, externalID string, deleteFiles bool) error {
	client, err := s.client(protocol)
	if err != nil {
		return err
	}
	return client.Remove(ctx, externalID, deleteFiles)
}
