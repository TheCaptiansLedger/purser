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
	id         string
	err        error
}

func (f *fakeJobPublisher) Trigger(_ context.Context, kind string, taskLabels []string) (string, error) {
	f.kind = kind
	f.taskLabels = taskLabels
	if f.err != nil {
		return "", f.err
	}
	return f.id, nil
}

type fakeJobReader struct {
	job *jobqueue.Job
	err error
}

func (f *fakeJobReader) Get(_ context.Context, _ string) (*jobqueue.Job, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.job, nil
}

func TestJobService_Trigger(t *testing.T) {
	pub := &fakeJobPublisher{id: "job-1"}
	svc := service.NewJobService(pub, &fakeJobReader{})

	id, err := svc.Trigger(context.Background(), "diagnostic", []string{"a", "b"})
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
}

func TestJobService_Trigger_Error(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeJobPublisher{err: wantErr}
	svc := service.NewJobService(pub, &fakeJobReader{})

	if _, err := svc.Trigger(context.Background(), "diagnostic", nil); !errors.Is(err, wantErr) {
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
