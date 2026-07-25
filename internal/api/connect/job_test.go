package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/pkg/jobqueue"
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
