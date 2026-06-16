package domain

import (
	"testing"
	"time"
)

func TestJob_ZeroValue(t *testing.T) {
	var j Job
	if j.Status != "" {
		t.Errorf("zero-value Status = %q, want empty string", j.Status)
	}
	if j.IsTerminal() {
		t.Error("IsTerminal() = true for zero-value job, want false")
	}
	if j.IsPending() {
		t.Error("IsPending() = true for zero-value job, want false")
	}
}

func TestJob_IsTerminal(t *testing.T) {
	cases := []struct {
		status JobStatus
		want   bool
	}{
		{JobStatusQueued, false},
		{JobStatusRunning, false},
		{JobStatusCompleted, true},
		{JobStatusFailed, true},
		{JobStatusCancelled, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			j := &Job{Status: tc.status}
			if got := j.IsTerminal(); got != tc.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJob_IsPending(t *testing.T) {
	cases := []struct {
		status JobStatus
		want   bool
	}{
		{JobStatusQueued, true},
		{JobStatusRunning, true},
		{JobStatusCompleted, false},
		{JobStatusFailed, false},
		{JobStatusCancelled, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			j := &Job{Status: tc.status}
			if got := j.IsPending(); got != tc.want {
				t.Errorf("IsPending() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJob_TimeFields(t *testing.T) {
	now := time.Now().UTC()
	j := &Job{StartedAt: &now, CompletedAt: &now}
	if j.StartedAt == nil {
		t.Error("StartedAt is nil after assignment")
	}
	if j.CompletedAt == nil {
		t.Error("CompletedAt is nil after assignment")
	}

	j2 := &Job{}
	if j2.StartedAt != nil {
		t.Error("StartedAt should be nil when not set")
	}
	if j2.CompletedAt != nil {
		t.Error("CompletedAt should be nil when not set")
	}
}
