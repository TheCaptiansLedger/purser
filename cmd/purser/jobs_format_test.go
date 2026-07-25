package main

import (
	"errors"
	"strings"
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

func TestSplitPaneHeights(t *testing.T) {
	listH, detailH := splitPaneHeights(40)
	if listH <= 0 || detailH <= 0 {
		t.Fatalf("splitPaneHeights(40) = (%d, %d), want both positive", listH, detailH)
	}
	if listH >= detailH {
		t.Errorf("list pane (%d) should be shorter than detail pane (%d)", listH, detailH)
	}
	if listH+detailH != 40 {
		t.Errorf("listH+detailH = %d, want 40 (the two panes should fill the body exactly)", listH+detailH)
	}

	// Very short terminals still get a usable (non-negative) split.
	listH, detailH = splitPaneHeights(3)
	if listH < 0 || detailH < 0 {
		t.Fatalf("splitPaneHeights(3) = (%d, %d), want both non-negative", listH, detailH)
	}
}

func TestPaneViewportHeight(t *testing.T) {
	if got := paneViewportHeight(10); got != 7 {
		t.Errorf("paneViewportHeight(10) = %d, want 7 (10 - 2 border - 1 title)", got)
	}
	if got := paneViewportHeight(0); got != 1 {
		t.Errorf("paneViewportHeight(0) = %d, want 1 (clamped to a usable minimum)", got)
	}
}

func TestRenderJobRow(t *testing.T) {
	job := &jobv1.Job{Id: "job-1", Kind: "diagnostic", Status: jobv1.JobStatus_JOB_STATUS_RUNNING, Progress: 0.5}

	unselected := renderJobRow(job, false)
	selected := renderJobRow(job, true)

	if !strings.Contains(unselected, "job-1") {
		t.Errorf("row should contain the job id, got %q", unselected)
	}
	if unselected == selected {
		t.Error("selected row should render differently from an unselected row")
	}
}

func TestRenderTaskRow(t *testing.T) {
	task := &jobv1.Task{Label: "track 1", Status: jobv1.JobStatus_JOB_STATUS_RUNNING, Progress: 0.25}

	unselected := renderTaskRow(task, false)
	selected := renderTaskRow(task, true)

	if !strings.Contains(unselected, "track 1") {
		t.Errorf("row should contain the task label, got %q", unselected)
	}
	if unselected == selected {
		t.Error("selected row should render differently from an unselected row")
	}
}

func TestRenderStepRow(t *testing.T) {
	step := &jobv1.Step{Name: "hash", Status: jobv1.JobStatus_JOB_STATUS_SUCCEEDED, Message: "ok"}

	got := renderStepRow(step)
	if !strings.Contains(got, "hash") || !strings.Contains(got, "ok") {
		t.Errorf("renderStepRow = %q, want it to contain the step name and message", got)
	}
}

func TestRenderHeader(t *testing.T) {
	got := renderHeader("http://localhost:7474", nil)
	if !strings.Contains(got, "http://localhost:7474") {
		t.Errorf("renderHeader should contain the addr, got %q", got)
	}

	withErr := renderHeader("http://localhost:7474", errors.New("boom"))
	if !strings.Contains(withErr, "boom") {
		t.Errorf("renderHeader should surface the list error, got %q", withErr)
	}
}

func TestRenderFooter(t *testing.T) {
	list := renderFooter(paneList, nil, false)
	detail := renderFooter(paneDetail, nil, false)
	if list == detail {
		t.Error("footer hint should differ between list and detail focus")
	}

	withStreamErr := renderFooter(paneDetail, errors.New("stream broke"), false)
	if !strings.Contains(withStreamErr, "stream broke") {
		t.Errorf("renderFooter should surface a stream error, got %q", withStreamErr)
	}

	closed := renderFooter(paneDetail, nil, true)
	if !strings.Contains(closed, "closed") {
		t.Errorf("renderFooter should note a closed stream, got %q", closed)
	}
}
