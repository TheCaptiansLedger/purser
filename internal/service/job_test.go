package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"purser/pkg/jobqueue"
	"testing"
)

type fakeJobPublisher struct {
	kind       string
	taskLabels []string
	params     map[string]string
	id         string
	err        error
}

func (f *fakeJobPublisher) Trigger(_ context.Context, kind string, taskLabels []string, params map[string]string) (string, error) {
	f.kind = kind
	f.taskLabels = taskLabels
	f.params = params
	if f.err != nil {
		return "", f.err
	}
	return f.id, nil
}

type fakeJobReader struct {
	job *jobqueue.Job
	err error

	listJobs          []*jobqueue.Job
	listNextPageToken string
	listErr           error
	listKind          string
	listStatus        jobqueue.Status
	listPageSize      int
	listPageToken     string

	watchEvents       []*jobqueue.Event
	watchErr          error
	watchUnsubscribed bool
}

func (f *fakeJobReader) Get(_ context.Context, _ string) (*jobqueue.Job, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.job, nil
}

func (f *fakeJobReader) List(_ context.Context, kind string, status jobqueue.Status, pageSize int, pageToken string) ([]*jobqueue.Job, string, error) {
	f.listKind = kind
	f.listStatus = status
	f.listPageSize = pageSize
	f.listPageToken = pageToken
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listJobs, f.listNextPageToken, nil
}

func (f *fakeJobReader) Watch(_ context.Context, _ string) (<-chan *jobqueue.Event, func(), error) {
	if f.watchErr != nil {
		return nil, nil, f.watchErr
	}
	ch := make(chan *jobqueue.Event, len(f.watchEvents))
	for _, e := range f.watchEvents {
		ch <- e
	}
	close(ch)
	return ch, func() { f.watchUnsubscribed = true }, nil
}

func TestJobService_Trigger(t *testing.T) {
	pub := &fakeJobPublisher{id: "job-1"}
	svc := service.NewJobService(pub, &fakeJobReader{})

	id, err := svc.Trigger(context.Background(), "diagnostic", []string{"a", "b"}, map[string]string{"fail_at_step:a": "1"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if id != "job-1" {
		t.Fatalf("Trigger returned id %q, want %q", id, "job-1")
	}
	if pub.kind != "diagnostic" {
		t.Fatalf("Trigger passed kind %q, want %q", pub.kind, "diagnostic")
	}
	if len(pub.taskLabels) != 2 {
		t.Fatalf("Trigger passed %d task labels, want 2", len(pub.taskLabels))
	}
	if pub.params["fail_at_step:a"] != "1" {
		t.Fatalf("Trigger passed params %v, want fail_at_step:a=1", pub.params)
	}
}

func TestJobService_Trigger_Error(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeJobPublisher{err: wantErr}
	svc := service.NewJobService(pub, &fakeJobReader{})

	if _, err := svc.Trigger(context.Background(), "diagnostic", nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("Trigger returned %v, want %v", err, wantErr)
	}
}

func TestJobService_Get(t *testing.T) {
	job := &jobqueue.Job{ID: "job-1", Status: jobqueue.StatusSucceeded}
	svc := service.NewJobService(&fakeJobPublisher{}, &fakeJobReader{job: job})

	got, err := svc.Get(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != job {
		t.Fatal("Get did not return the reader's job")
	}
}

func TestJobService_Get_NotFound(t *testing.T) {
	svc := service.NewJobService(&fakeJobPublisher{}, &fakeJobReader{err: ports.ErrNotFound})

	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get returned %v, want ports.ErrNotFound", err)
	}
}

func TestJobService_List(t *testing.T) {
	jobs := []*jobqueue.Job{{ID: "job-1"}, {ID: "job-2"}}
	reader := &fakeJobReader{listJobs: jobs, listNextPageToken: "job-2"}
	svc := service.NewJobService(&fakeJobPublisher{}, reader)

	got, next, err := svc.List(context.Background(), "diagnostic", jobqueue.StatusPartial, 2, "token")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 2 || next != "job-2" {
		t.Fatalf("List returned (%v, %q), want (%v, %q)", got, next, jobs, "job-2")
	}
	if reader.listKind != "diagnostic" || reader.listStatus != jobqueue.StatusPartial || reader.listPageSize != 2 || reader.listPageToken != "token" {
		t.Fatalf("List passed (kind=%q, status=%q, pageSize=%d, pageToken=%q), want (diagnostic, partial, 2, token)",
			reader.listKind, reader.listStatus, reader.listPageSize, reader.listPageToken)
	}
}

func TestJobService_List_Error(t *testing.T) {
	wantErr := errors.New("boom")
	svc := service.NewJobService(&fakeJobPublisher{}, &fakeJobReader{listErr: wantErr})

	if _, _, err := svc.List(context.Background(), "", "", 0, ""); !errors.Is(err, wantErr) {
		t.Fatalf("List returned %v, want %v", err, wantErr)
	}
}

func TestJobService_Watch(t *testing.T) {
	events := []*jobqueue.Event{
		{Kind: jobqueue.EventKindJob, Job: &jobqueue.Job{ID: "job-1", Status: jobqueue.StatusRunning}},
		{Kind: jobqueue.EventKindJob, Job: &jobqueue.Job{ID: "job-1", Status: jobqueue.StatusSucceeded}},
	}
	reader := &fakeJobReader{watchEvents: events}
	svc := service.NewJobService(&fakeJobPublisher{}, reader)

	ch, unsubscribe, err := svc.Watch(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Watch returned error: %v", err)
	}

	var got []*jobqueue.Event
	for e := range ch {
		got = append(got, e)
	}
	if len(got) != 2 || got[0] != events[0] || got[1] != events[1] {
		t.Fatalf("Watch delivered %v, want %v", got, events)
	}

	unsubscribe()
	if !reader.watchUnsubscribed {
		t.Fatal("Watch's unsubscribe did not call through to the reader's")
	}
}

func TestJobService_Watch_Error(t *testing.T) {
	wantErr := errors.New("boom")
	svc := service.NewJobService(&fakeJobPublisher{}, &fakeJobReader{watchErr: wantErr})

	if _, _, err := svc.Watch(context.Background(), "job-1"); !errors.Is(err, wantErr) {
		t.Fatalf("Watch returned %v, want %v", err, wantErr)
	}
}
