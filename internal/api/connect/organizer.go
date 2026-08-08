package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/pipeline/v1/pipelinev1connect"
	"purser/internal/domain"

	"connectrpc.com/connect"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"
)

// organizerService is the narrow interface OrganizerHandler depends on —
// see personService for the DIP convention this follows.
type organizerService interface {
	Organize(ctx context.Context, mediaFileID string) (*domain.MediaFile, error)
}

// OrganizerHandler implements pipelinev1connect.OrganizerServiceHandler —
// the manual-trigger surface for docs/adr/0024-pipeline-core.md's Organizer
// stage, always available regardless of config.Pipeline.AutoOrganize. See
// docs/technical/pipeline-music-organizer.md.
type OrganizerHandler struct {
	pipelinev1connect.UnimplementedOrganizerServiceHandler
	svc    organizerService
	logger *slog.Logger
}

// NewOrganizerHandler constructs an OrganizerHandler backed by svc.
func NewOrganizerHandler(svc organizerService, logger *slog.Logger) *OrganizerHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &OrganizerHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "OrganizerService")}
}

// Organize implements pipelinev1connect.OrganizerServiceHandler. A
// collision (ports.ErrDestinationExists) or an escaped-root
// (ports.ErrDestinationOutsideRoot) failure surfaces directly to the
// caller via mapError, per docs/technical/pipeline-music-organizer.md's
// "on the manual RPC path, that error surfaces directly to the caller."
func (h *OrganizerHandler) Organize(ctx context.Context, req *connect.Request[pipelinev1.OrganizeRequest]) (*connect.Response[pipelinev1.OrganizeResponse], error) {
	mf, err := h.svc.Organize(ctx, req.Msg.GetMediaFileId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&pipelinev1.OrganizeResponse{MediaFile: mediaFileToProto(mf)}), nil
}
