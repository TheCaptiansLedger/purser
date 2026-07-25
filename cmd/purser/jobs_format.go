package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
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

// Layout constants. The screen is split into a header, a two-pane body
// stacked vertically (list on top, detail below — each pane gets the full
// terminal width so long id/status/message columns don't get squeezed into
// a narrow half-screen column), and a footer.
const (
	jobsHeaderHeight   = 1
	jobsFooterHeight   = 1
	jobsPaneTitleRows  = 1 // one title line rendered inside each bordered pane
	jobsListHeightFrac = 0.4
)

var (
	jobsBorderColorFocused   = lipgloss.Color("212")
	jobsBorderColorUnfocused = lipgloss.Color("240")

	jobsPaneTitleStyle = lipgloss.NewStyle().Bold(true)

	jobsHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	jobsFooterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	jobsErrorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("204"))

	jobsListHeaderRowStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	jobsSelectedRowStyle   = lipgloss.NewStyle().Bold(true).Reverse(true)
	jobsSelectedTaskStyle  = lipgloss.NewStyle().Bold(true).Reverse(true)
)

// paneStyle returns the bordered container style for a pane, highlighted
// when it has keyboard focus.
func paneStyle(width, height int, focused bool) lipgloss.Style {
	color := jobsBorderColorUnfocused
	if focused {
		color = jobsBorderColorFocused
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Height(height)
}

// renderPane wraps a title and body into a bordered pane of the given
// interior width/height (interior = content area excluding the border).
func renderPane(title string, body string, width, height int, focused bool) string {
	titleLine := jobsPaneTitleStyle.Render(title)
	content := titleLine + "\n" + body
	return paneStyle(width, height, focused).Render(content)
}

// renderHeader renders the top title/status bar.
func renderHeader(addr string, listErr error) string {
	left := jobsHeaderStyle.Render("purser jobs") + jobsFooterStyle.Render("  "+addr)
	if listErr != nil {
		return left + "   " + jobsErrorStyle.Render(fmt.Sprintf("list error: %v", listErr))
	}
	return left
}

// renderFooter renders the contextual keybinding hint bar.
func renderFooter(focus pane, streamErr error, streamDone bool) string {
	var hint string
	switch focus {
	case paneDetail:
		hint = "tab: focus list  ↑/k ↓/j: select task  pgup/pgdn: scroll  q: quit"
	default:
		hint = "tab: focus detail  ↑/k ↓/j: select job  r: refresh  q: quit"
	}
	switch {
	case streamErr != nil:
		return jobsFooterStyle.Render(hint) + "   " + jobsErrorStyle.Render(fmt.Sprintf("stream error: %v", streamErr))
	case streamDone:
		return jobsFooterStyle.Render(hint) + "   " + jobsFooterStyle.Render("(stream closed)")
	default:
		return jobsFooterStyle.Render(hint)
	}
}

// selectionMarker prefixes a row with a plain-text cursor indicator, so the
// highlighted row is still distinguishable when the terminal doesn't render
// the accompanying style (e.g. NO_COLOR, non-TTY output).
func selectionMarker(selected bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

// renderJobRow renders one row of the job list table.
func renderJobRow(job *jobv1.Job, selected bool) string {
	row := selectionMarker(selected) + fmt.Sprintf("%-36s %-16s %-11s %s", job.GetId(), job.GetKind(), jobStatusLabel(job.GetStatus()), formatProgress(job.GetProgress()))
	if selected {
		return jobsSelectedRowStyle.Render(row)
	}
	return row
}

// renderJobListHeaderRow renders the column header row above the job table.
func renderJobListHeaderRow() string {
	return jobsListHeaderRowStyle.Render(fmt.Sprintf("  %-36s %-16s %-11s %s", "ID", "KIND", "STATUS", "PROG"))
}

// renderTaskRow renders one row of a job's task table.
func renderTaskRow(task *jobv1.Task, selected bool) string {
	row := selectionMarker(selected) + fmt.Sprintf("%-40s %-11s %s", task.GetLabel(), jobStatusLabel(task.GetStatus()), formatProgress(task.GetProgress()))
	if selected {
		return jobsSelectedTaskStyle.Render(row)
	}
	return row
}

// renderStepRow renders one row of a task's step table.
func renderStepRow(step *jobv1.Step) string {
	return fmt.Sprintf("    %-24s %-11s %-10s %s", step.GetName(), jobStatusLabel(step.GetStatus()), formatDuration(step.GetStartedAt(), step.GetFinishedAt()), step.GetMessage())
}

// splitPaneHeights splits the available body height between the two
// stacked panes' outer (border-inclusive) heights, reserving room for both
// panes' borders (2 rows each) and guarding against negative heights on
// very short terminals.
func splitPaneHeights(totalHeight int) (listOuter, detailOuter int) {
	if totalHeight < 4 {
		return totalHeight, 0
	}
	const minOuter = 4 // border(2) + title(1) + at least 1 content row
	listOuter = int(float64(totalHeight) * jobsListHeightFrac)
	if listOuter < minOuter {
		listOuter = minOuter
	}
	if listOuter > totalHeight-2 {
		listOuter = totalHeight - 2
	}
	detailOuter = totalHeight - listOuter
	return listOuter, detailOuter
}

// paneViewportHeight converts a pane's outer (border-inclusive) height into
// the interior row budget available to its viewport, after the border and
// title rows are accounted for.
func paneViewportHeight(outer int) int {
	h := outer - 2 - jobsPaneTitleRows
	if h < 1 {
		return 1
	}
	return h
}
