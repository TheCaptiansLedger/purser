package apiconnect

import (
	"purser/pkg/jobqueue"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	jobv1 "purser/gen/go/purser/job/v1"
)

func jobStatusToProto(s jobqueue.Status) jobv1.JobStatus {
	switch s {
	case jobqueue.StatusPending:
		return jobv1.JobStatus_JOB_STATUS_PENDING
	case jobqueue.StatusRunning:
		return jobv1.JobStatus_JOB_STATUS_RUNNING
	case jobqueue.StatusSucceeded:
		return jobv1.JobStatus_JOB_STATUS_SUCCEEDED
	case jobqueue.StatusFailed:
		return jobv1.JobStatus_JOB_STATUS_FAILED
	case jobqueue.StatusPartial:
		return jobv1.JobStatus_JOB_STATUS_PARTIAL
	default:
		return jobv1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

// protoToJobStatus is the reverse of jobStatusToProto, used to convert a
// ListJobs request's status filter. JOB_STATUS_UNSPECIFIED (and any
// unrecognized value) maps to "" — no filter on that dimension.
func protoToJobStatus(s jobv1.JobStatus) jobqueue.Status {
	switch s {
	case jobv1.JobStatus_JOB_STATUS_PENDING:
		return jobqueue.StatusPending
	case jobv1.JobStatus_JOB_STATUS_RUNNING:
		return jobqueue.StatusRunning
	case jobv1.JobStatus_JOB_STATUS_SUCCEEDED:
		return jobqueue.StatusSucceeded
	case jobv1.JobStatus_JOB_STATUS_FAILED:
		return jobqueue.StatusFailed
	case jobv1.JobStatus_JOB_STATUS_PARTIAL:
		return jobqueue.StatusPartial
	default:
		return ""
	}
}

// timestampToProto returns nil for a zero Time (not yet started/finished)
// rather than a Timestamp pointing at the Unix epoch.
func timestampToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func stepToProto(s *jobqueue.Step) *jobv1.Step {
	if s == nil {
		return nil
	}
	return &jobv1.Step{
		Id:         s.ID,
		Name:       s.Name,
		Status:     jobStatusToProto(s.Status),
		StartedAt:  timestampToProto(s.StartedAt),
		FinishedAt: timestampToProto(s.FinishedAt),
		Message:    s.Message,
		Detail:     s.Detail,
	}
}

func taskToProto(t *jobqueue.Task) *jobv1.Task {
	if t == nil {
		return nil
	}
	steps := make([]*jobv1.Step, 0, len(t.Steps))
	for _, s := range t.Steps {
		steps = append(steps, stepToProto(s))
	}
	return &jobv1.Task{
		Id:         t.ID,
		Label:      t.Label,
		Status:     jobStatusToProto(t.Status),
		StartedAt:  timestampToProto(t.StartedAt),
		FinishedAt: timestampToProto(t.FinishedAt),
		Steps:      steps,
		Progress:   t.Progress(),
	}
}

func jobEventKindToProto(k jobqueue.EventKind) jobv1.JobEventKind {
	switch k {
	case jobqueue.EventKindJob:
		return jobv1.JobEventKind_JOB_EVENT_KIND_JOB
	case jobqueue.EventKindTask:
		return jobv1.JobEventKind_JOB_EVENT_KIND_TASK
	case jobqueue.EventKindStep:
		return jobv1.JobEventKind_JOB_EVENT_KIND_STEP
	default:
		return jobv1.JobEventKind_JOB_EVENT_KIND_UNSPECIFIED
	}
}

func jobEventToProto(e *jobqueue.Event) *jobv1.JobEvent {
	if e == nil {
		return nil
	}
	return &jobv1.JobEvent{
		Kind:   jobEventKindToProto(e.Kind),
		TaskId: e.TaskID,
		StepId: e.StepID,
		Job:    jobToProto(e.Job),
	}
}

func jobToProto(j *jobqueue.Job) *jobv1.Job {
	if j == nil {
		return nil
	}
	tasks := make([]*jobv1.Task, 0, len(j.Tasks))
	for _, t := range j.Tasks {
		tasks = append(tasks, taskToProto(t))
	}
	return &jobv1.Job{
		Id:         j.ID,
		Kind:       j.Kind,
		Status:     jobStatusToProto(j.Status),
		CreatedAt:  timestampToProto(j.CreatedAt),
		StartedAt:  timestampToProto(j.StartedAt),
		FinishedAt: timestampToProto(j.FinishedAt),
		Tasks:      tasks,
		Progress:   j.Progress(),
		Params:     j.Params,
	}
}
