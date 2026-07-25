package main

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"google.golang.org/protobuf/types/known/timestamppb"

	jobv1 "purser/gen/go/purser/job/v1"
)

// jobStatusLabel renders status as a short, colored label. Coloring here is
// pure string formatting via pterm's Sprint helpers, not interactive
// pterm I/O — the TUI's control flow stays entirely Bubble Tea, per
// docs/adr/0009-cli-stack.md's "don't mix both for the same interaction"
// rule.
func jobStatusLabel(status jobv1.JobStatus) string {
	switch status {
	case jobv1.JobStatus_JOB_STATUS_PENDING:
		return pterm.Gray("pending")
	case jobv1.JobStatus_JOB_STATUS_RUNNING:
		return pterm.LightYellow("running")
	case jobv1.JobStatus_JOB_STATUS_SUCCEEDED:
		return pterm.LightGreen("succeeded")
	case jobv1.JobStatus_JOB_STATUS_FAILED:
		return pterm.LightRed("failed")
	case jobv1.JobStatus_JOB_STATUS_PARTIAL:
		return pterm.Yellow("partial")
	default:
		return pterm.Gray("unknown")
	}
}

// formatProgress renders a 0..1 fraction as a percentage.
func formatProgress(progress float64) string {
	return fmt.Sprintf("%3.0f%%", progress*100)
}

// formatDuration renders the elapsed time between start and finish. If
// finish is nil, elapsed time is measured against now (the task/job is
// still in flight). If start is nil, nothing has happened yet.
func formatDuration(start, finish *timestamppb.Timestamp) string {
	if start == nil {
		return "-"
	}
	end := time.Now()
	if finish != nil {
		end = finish.AsTime()
	}
	return end.Sub(start.AsTime()).Round(time.Second).String()
}
