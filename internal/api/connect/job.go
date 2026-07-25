package apiconnect

import (
	"context"
	"log/slog"
	"purser/gen/go/purser/job/v1/jobv1connect"
	"purser/pkg/jobqueue"

	"connectrpc.com/connect"

	jobv1 "purser/gen/go/purser/job/v1"
)

// jobService is the narrow interface JobHandler depends on — see
// personService for the DIP convention this follows.
type jobService interface {
	Trigger(ctx context.Context, kind string, taskLabels []string, params map[string]string) (string, error)
	Get(ctx context.Context, id string) (*jobqueue.Job, error)
	List(ctx context.Context, kind string, status jobqueue.Status, pageSize int, pageToken string) ([]*jobqueue.Job, string, error)
}

// JobHandler implements jobv1connect.JobServiceHandler. Job isn't
// Datastore-backed (see docs/adr/0023-job-queue.md), but the handler
// follows the same conventions as every other entity's per
// docs/adr/0011-api-design.md.
type JobHandler struct {
	jobv1connect.UnimplementedJobServiceHandler
	svc    jobService
	logger *slog.Logger
}

// NewJobHandler constructs a JobHandler backed by svc.
func NewJobHandler(svc jobService, logger *slog.Logger) *JobHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &JobHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "JobService")}
}

// TriggerJob implements jobv1connect.JobServiceHandler.
func (h *JobHandler) TriggerJob(ctx context.Context, req *connect.Request[jobv1.TriggerJobRequest]) (*connect.Response[jobv1.TriggerJobResponse], error) {
	id, err := h.svc.Trigger(ctx, req.Msg.GetKind(), req.Msg.GetTaskLabels(), req.Msg.GetParams())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&jobv1.TriggerJobResponse{JobId: id}), nil
}

// GetJob implements jobv1connect.JobServiceHandler.
func (h *JobHandler) GetJob(ctx context.Context, req *connect.Request[jobv1.GetJobRequest]) (*connect.Response[jobv1.GetJobResponse], error) {
	job, err := h.svc.Get(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(&jobv1.GetJobResponse{Job: jobToProto(job)}), nil
}

// ListJobs implements jobv1connect.JobServiceHandler.
func (h *JobHandler) ListJobs(ctx context.Context, req *connect.Request[jobv1.ListJobsRequest]) (*connect.Response[jobv1.ListJobsResponse], error) {
	jobs, next, err := h.svc.List(ctx, req.Msg.GetKind(), protoToJobStatus(req.Msg.GetStatus()), int(req.Msg.GetPageSize()), req.Msg.GetPageToken())
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	pbJobs := make([]*jobv1.Job, 0, len(jobs))
	for _, j := range jobs {
		pbJobs = append(pbJobs, jobToProto(j))
	}
	return connect.NewResponse(&jobv1.ListJobsResponse{Jobs: pbJobs, NextPageToken: next}), nil
}
