package apiconnect_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/gen/go/purser/job/v1/jobv1connect"
	"purser/internal/ports"
	"purser/pkg/jobqueue"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	jobv1 "purser/gen/go/purser/job/v1"

	apiconnect "purser/internal/api/connect"
)

type fakeJobService struct {
	triggerKind   string
	triggerLabels []string
	triggerParams map[string]string
	triggerID     string
	triggerErr    error

	getJob *jobqueue.Job
	getErr error

	listJobs          []*jobqueue.Job
	listNextPageToken string
	listErr           error
	listKind          string
	listStatus        jobqueue.Status
	listPageSize      int
	listPageToken     string

	watchCh    <-chan *jobqueue.Event
	watchUnsub func()
	watchErr   error
}

func (f *fakeJobService) Trigger(_ context.Context, kind string, taskLabels []string, params map[string]string) (string, error) {
	f.triggerKind = kind
	f.triggerLabels = taskLabels
	f.triggerParams = params
	if f.triggerErr != nil {
		return "", f.triggerErr
	}
	return f.triggerID, nil
}

func (f *fakeJobService) Get(_ context.Context, _ string) (*jobqueue.Job, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getJob, nil
}

func (f *fakeJobService) List(_ context.Context, kind string, status jobqueue.Status, pageSize int, pageToken string) ([]*jobqueue.Job, string, error) {
	f.listKind = kind
	f.listStatus = status
	f.listPageSize = pageSize
	f.listPageToken = pageToken
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listJobs, f.listNextPageToken, nil
}

func (f *fakeJobService) Watch(_ context.Context, _ string) (<-chan *jobqueue.Event, func(), error) {
	if f.watchErr != nil {
		return nil, nil, f.watchErr
	}
	return f.watchCh, f.watchUnsub, nil
}

// newWatchTestServer mounts h behind a real HTTP server — WatchJob's third
// parameter, *connect.ServerStream, has no exported constructor, so unlike
// this file's other (direct in-process call) tests, a streaming handler
// can only be exercised through a real client/server round-trip. See
// docs/adr/0023-job-queue.md and docs/adr/0011-api-design.md.
func newWatchTestServer(t *testing.T, h *apiconnect.JobHandler) jobv1connect.JobServiceClient {
	t.Helper()
	path, handler := jobv1connect.NewJobServiceHandler(h)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return jobv1connect.NewJobServiceClient(server.Client(), server.URL)
}

func TestJobHandler_WatchJob(t *testing.T) {
	events := make(chan *jobqueue.Event, 2)
	events <- &jobqueue.Event{Kind: jobqueue.EventKindJob, Job: &jobqueue.Job{ID: "job-1", Status: jobqueue.StatusRunning}}
	events <- &jobqueue.Event{Kind: jobqueue.EventKindTask, TaskID: "task-1", Job: &jobqueue.Job{ID: "job-1", Status: jobqueue.StatusSucceeded}}
	close(events)

	var unsubscribed atomic.Bool
	svc := &fakeJobService{watchCh: events, watchUnsub: func() { unsubscribed.Store(true) }}
	client := newWatchTestServer(t, apiconnect.NewJobHandler(svc, nil))

	stream, err := client.WatchJob(context.Background(), connect.NewRequest(&jobv1.WatchJobRequest{JobId: "job-1"}))
	if err != nil {
		t.Fatalf("WatchJob returned error: %v", err)
	}

	var got []*jobv1.JobEvent
	for stream.Receive() {
		got = append(got, stream.Msg())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream ended with error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("received %d events, want 2", len(got))
	}
	if got[0].GetJob().GetStatus() != jobv1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("first event status = %v, want RUNNING", got[0].GetJob().GetStatus())
	}
	if got[1].GetKind() != jobv1.JobEventKind_JOB_EVENT_KIND_TASK || got[1].GetTaskId() != "task-1" {
		t.Fatalf("second event = %+v, want kind=TASK task_id=task-1", got[1])
	}
	if got[1].GetJob().GetStatus() != jobv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("second event status = %v, want SUCCEEDED", got[1].GetJob().GetStatus())
	}
	if !unsubscribed.Load() {
		t.Fatal("WatchJob did not unsubscribe after the event channel closed")
	}
}

func TestJobHandler_WatchJob_NotFound(t *testing.T) {
	svc := &fakeJobService{watchErr: ports.ErrNotFound}
	client := newWatchTestServer(t, apiconnect.NewJobHandler(svc, nil))

	stream, err := client.WatchJob(context.Background(), connect.NewRequest(&jobv1.WatchJobRequest{JobId: "missing"}))
	if err != nil {
		t.Fatalf("WatchJob returned error: %v", err)
	}
	if stream.Receive() {
		t.Fatal("stream.Receive() returned true, want the stream to end immediately with an error")
	}
	var connErr *connect.Error
	if !errors.As(stream.Err(), &connErr) {
		t.Fatalf("stream ended with %v, want a *connect.Error", stream.Err())
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("stream ended with code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}

// TestJobHandler_WatchJob_ClientDisconnect demonstrates the acceptance
// criterion that cancelling the client context stops the handler from
// waiting on further events and releases its subscription — never events
// arrive here, so the only way the stream can end is via ctx.Done().
func TestJobHandler_WatchJob_ClientDisconnect(t *testing.T) {
	events := make(chan *jobqueue.Event) // deliberately never sent to or closed
	var unsubscribed atomic.Bool
	svc := &fakeJobService{watchCh: events, watchUnsub: func() { unsubscribed.Store(true) }}
	client := newWatchTestServer(t, apiconnect.NewJobHandler(svc, nil))

	ctx, cancel := context.WithCancel(context.Background())

	// The server never sends anything in this scenario (no events ever
	// arrive), so response headers never flush and the client call blocks
	// waiting on them — cancel must happen concurrently with, not after,
	// the blocking client call, or it would never be reached.
	streamEnded := make(chan struct{})
	go func() {
		defer close(streamEnded)
		stream, err := client.WatchJob(ctx, connect.NewRequest(&jobv1.WatchJobRequest{JobId: "job-1"}))
		if err != nil {
			return
		}
		for stream.Receive() {
		}
	}()

	// Give the stream a moment to actually establish server-side before
	// cancelling, so this exercises a live subscription, not a request
	// that never reached the handler.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-streamEnded:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not end after the client cancelled its context")
	}

	deadline := time.Now().Add(2 * time.Second)
	for !unsubscribed.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !unsubscribed.Load() {
		t.Fatal("WatchJob did not unsubscribe after the client disconnected")
	}
}

func TestJobHandler_TriggerJob(t *testing.T) {
	svc := &fakeJobService{triggerID: "job-1"}
	h := apiconnect.NewJobHandler(svc, nil)

	resp, err := h.TriggerJob(context.Background(), connect.NewRequest(&jobv1.TriggerJobRequest{
		Kind:       "diagnostic",
		TaskLabels: []string{"a", "b"},
		Params:     map[string]string{"fail_at_step:a": "1"},
	}))
	if err != nil {
		t.Fatalf("TriggerJob returned error: %v", err)
	}
	if resp.Msg.GetJobId() != "job-1" {
		t.Fatalf("TriggerJob returned job_id %q, want %q", resp.Msg.GetJobId(), "job-1")
	}
	if svc.triggerKind != "diagnostic" {
		t.Fatalf("TriggerJob passed kind %q, want %q", svc.triggerKind, "diagnostic")
	}
	if len(svc.triggerLabels) != 2 {
		t.Fatalf("TriggerJob passed %d task labels, want 2", len(svc.triggerLabels))
	}
	if svc.triggerParams["fail_at_step:a"] != "1" {
		t.Fatalf("TriggerJob passed params %v, want fail_at_step:a=1", svc.triggerParams)
	}
}

func TestJobHandler_TriggerJob_UnknownKind(t *testing.T) {
	svc := &fakeJobService{triggerErr: jobqueue.ErrUnknownKind}
	h := apiconnect.NewJobHandler(svc, nil)

	_, err := h.TriggerJob(context.Background(), connect.NewRequest(&jobv1.TriggerJobRequest{Kind: "nonexistent"}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("TriggerJob returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("TriggerJob returned code %v, want %v", connErr.Code(), connect.CodeInvalidArgument)
	}
}

func TestJobHandler_GetJob(t *testing.T) {
	now := time.Now()
	job := &jobqueue.Job{
		ID:        "job-1",
		Kind:      "diagnostic",
		Status:    jobqueue.StatusSucceeded,
		CreatedAt: now,
		Params:    map[string]string{"fail_at_step:a": "1"},
		Tasks: []*jobqueue.Task{
			{ID: "task-1", Label: "one", Status: jobqueue.StatusSucceeded, Steps: []*jobqueue.Step{
				{ID: "step-1", Name: "initialize", Status: jobqueue.StatusSucceeded, Detail: map[string]string{"elapsed_ms": "50"}},
			}},
		},
	}
	svc := &fakeJobService{getJob: job}
	h := apiconnect.NewJobHandler(svc, nil)

	resp, err := h.GetJob(context.Background(), connect.NewRequest(&jobv1.GetJobRequest{Id: "job-1"}))
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	got := resp.Msg.GetJob()
	if got.GetId() != "job-1" || got.GetKind() != "diagnostic" {
		t.Fatalf("GetJob returned unexpected job: %+v", got)
	}
	if got.GetStatus() != jobv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("GetJob returned status %v, want %v", got.GetStatus(), jobv1.JobStatus_JOB_STATUS_SUCCEEDED)
	}
	if got.GetProgress() != 1 {
		t.Fatalf("GetJob returned progress %v, want 1", got.GetProgress())
	}
	if len(got.GetTasks()) != 1 || len(got.GetTasks()[0].GetSteps()) != 1 {
		t.Fatalf("GetJob did not round-trip tasks/steps: %+v", got)
	}
	if got.GetTasks()[0].GetSteps()[0].GetDetail()["elapsed_ms"] != "50" {
		t.Fatalf("GetJob did not round-trip step detail: %+v", got.GetTasks()[0].GetSteps()[0])
	}
	if got.GetParams()["fail_at_step:a"] != "1" {
		t.Fatalf("GetJob did not round-trip job params: %+v", got.GetParams())
	}
}

func TestJobHandler_GetJob_NotFound(t *testing.T) {
	svc := &fakeJobService{getErr: ports.ErrNotFound}
	h := apiconnect.NewJobHandler(svc, nil)

	_, err := h.GetJob(context.Background(), connect.NewRequest(&jobv1.GetJobRequest{Id: "missing"}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("GetJob returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("GetJob returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}

func TestJobHandler_ListJobs(t *testing.T) {
	jobs := []*jobqueue.Job{
		{ID: "job-1", Kind: "diagnostic", Status: jobqueue.StatusPartial},
	}
	svc := &fakeJobService{listJobs: jobs, listNextPageToken: "job-1"}
	h := apiconnect.NewJobHandler(svc, nil)

	resp, err := h.ListJobs(context.Background(), connect.NewRequest(&jobv1.ListJobsRequest{
		PageSize:  2,
		PageToken: "prev-token",
		Kind:      "diagnostic",
		Status:    jobv1.JobStatus_JOB_STATUS_PARTIAL,
	}))
	if err != nil {
		t.Fatalf("ListJobs returned error: %v", err)
	}
	if len(resp.Msg.GetJobs()) != 1 || resp.Msg.GetJobs()[0].GetId() != "job-1" {
		t.Fatalf("ListJobs returned unexpected jobs: %+v", resp.Msg.GetJobs())
	}
	if resp.Msg.GetNextPageToken() != "job-1" {
		t.Fatalf("ListJobs returned next_page_token %q, want %q", resp.Msg.GetNextPageToken(), "job-1")
	}
	if svc.listKind != "diagnostic" || svc.listStatus != jobqueue.StatusPartial || svc.listPageSize != 2 || svc.listPageToken != "prev-token" {
		t.Fatalf("ListJobs passed (kind=%q, status=%q, pageSize=%d, pageToken=%q), want (diagnostic, partial, 2, prev-token)",
			svc.listKind, svc.listStatus, svc.listPageSize, svc.listPageToken)
	}
}

func TestJobHandler_ListJobs_NoFilter(t *testing.T) {
	svc := &fakeJobService{}
	h := apiconnect.NewJobHandler(svc, nil)

	_, err := h.ListJobs(context.Background(), connect.NewRequest(&jobv1.ListJobsRequest{}))
	if err != nil {
		t.Fatalf("ListJobs returned error: %v", err)
	}
	if svc.listKind != "" || svc.listStatus != "" {
		t.Fatalf("ListJobs passed (kind=%q, status=%q) for an unset request, want empty filters", svc.listKind, svc.listStatus)
	}
}

func TestJobHandler_ListJobs_Error(t *testing.T) {
	svc := &fakeJobService{listErr: ports.ErrNotFound}
	h := apiconnect.NewJobHandler(svc, nil)

	_, err := h.ListJobs(context.Background(), connect.NewRequest(&jobv1.ListJobsRequest{}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("ListJobs returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeNotFound {
		t.Fatalf("ListJobs returned code %v, want %v", connErr.Code(), connect.CodeNotFound)
	}
}
