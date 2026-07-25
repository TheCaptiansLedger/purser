package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/pipeline/v1/pipelinev1connect"

	"connectrpc.com/connect"

	pipelinev1 "purser/gen/go/purser/pipeline/v1"
)

// scanService is the narrow interface ScanHandler depends on — see
// personService for the DIP convention this follows.
type scanService interface {
	Trigger(ctx context.Context, root string) (string, error)
}

// ScanHandler implements pipelinev1connect.ScanServiceHandler. See
// docs/adr/0024-pipeline-core.md.
type ScanHandler struct {
	pipelinev1connect.UnimplementedScanServiceHandler
	svc    scanService
	logger *slog.Logger
}

// NewScanHandler constructs a ScanHandler backed by svc.
func NewScanHandler(svc scanService, logger *slog.Logger) *ScanHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "ScanService")}
}

// TriggerScan implements pipelinev1connect.ScanServiceHandler. Progress is
// read via JobService.GetJob/WatchJob using the returned job_id.
func (h *ScanHandler) TriggerScan(ctx context.Context, req *connect.Request[pipelinev1.TriggerScanRequest]) (*connect.Response[pipelinev1.TriggerScanResponse], error) {
	id, err := h.svc.Trigger(ctx, req.Msg.GetRoot())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&pipelinev1.TriggerScanResponse{JobId: id}), nil
}
