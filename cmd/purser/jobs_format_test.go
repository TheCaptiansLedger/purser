package main

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	jobv1 "purser/gen/go/purser/job/v1"
)

func TestJobStatusLabel(t *testing.T) {
	tests := []struct {
		name   string
		status jobv1.JobStatus
	}{
		{"pending", jobv1.JobStatus_JOB_STATUS_PENDING},
		{"running", jobv1.JobStatus_JOB_STATUS_RUNNING},
		{"succeeded", jobv1.JobStatus_JOB_STATUS_SUCCEEDED},
		{"failed", jobv1.JobStatus_JOB_STATUS_FAILED},
		{"partial", jobv1.JobStatus_JOB_STATUS_PARTIAL},
		{"unknown", jobv1.JobStatus_JOB_STATUS_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := jobStatusLabel(tt.status)
			if label == "" {
				t.Fatalf("jobStatusLabel(%v) returned empty string", tt.status)
			}
		})
	}
}

func TestFormatProgress(t *testing.T) {
	tests := []struct {
		progress float64
		want     string
	}{
		{0, "  0%"},
		{0.5, " 50%"},
		{1, "100%"},
	}
	for _, tt := range tests {
		if got := formatProgress(tt.progress); got != tt.want {
			t.Errorf("formatProgress(%v) = %q, want %q", tt.progress, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	start := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	finish := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 5, 0, time.UTC))

	if got := formatDuration(nil, nil); got != "-" {
		t.Errorf("formatDuration(nil, nil) = %q, want %q", got, "-")
	}
	if got := formatDuration(start, finish); got != "5s" {
		t.Errorf("formatDuration(start, finish) = %q, want %q", got, "5s")
	}
	if got := formatDuration(start, nil); got == "-" {
		t.Errorf("formatDuration(start, nil) should measure elapsed time against now, got %q", got)
	}
}
