package main

import (
	"context"
	"fmt"
	"purser/gen/go/purser/job/v1/jobv1connect"
	"strings"
	"time"

	"connectrpc.com/connect"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pterm/pterm"

	jobv1 "purser/gen/go/purser/job/v1"
)

// jobEventStream is the narrow subset of *connect.ServerStreamForClient
// the model depends on, so tests can substitute a fake stream instead of a
// live WatchJob RPC. See docs/adr/0002-solid-design-principles.md (DIP/ISP).
type jobEventStream interface {
	Receive() bool
	Msg() *jobv1.JobEvent
	Err() error
	Close() error
}

var _ jobEventStream = (*connect.ServerStreamForClient[jobv1.JobEvent])(nil)

type screen int

const (
	screenList screen = iota
	screenDetail
)

const (
	jobsRefreshInterval = 3 * time.Second
	jobsListPageSize    = 50
)

// jobsModel is the Bubble Tea model backing `purser jobs`. It has two
// screens: a polled list of recent jobs, and a streamed detail view of one
// selected job's tasks/steps.
type jobsModel struct {
	ctx    context.Context
	client jobv1connect.JobServiceClient

	screen screen

	jobs    []*jobv1.Job
	cursor  int
	listErr error

	selected   *jobv1.Job
	stream     jobEventStream
	taskCursor int
	streamErr  error
	streamDone bool
}

func newJobsModel(ctx context.Context, client jobv1connect.JobServiceClient) jobsModel {
	return jobsModel{ctx: ctx, client: client}
}

func (m jobsModel) Init() tea.Cmd {
	return tea.Batch(fetchJobsCmd(m.ctx, m.client), tickCmd())
}

type jobsLoadedMsg struct {
	jobs []*jobv1.Job
	err  error
}

type tickMsg struct{}

type streamOpenedMsg struct {
	stream jobEventStream
	err    error
}

type jobEventMsg struct {
	event *jobv1.JobEvent
}

type streamErrMsg struct {
	err error
}

type streamClosedMsg struct{}

func fetchJobsCmd(ctx context.Context, client jobv1connect.JobServiceClient) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.ListJobs(ctx, connect.NewRequest(&jobv1.ListJobsRequest{PageSize: jobsListPageSize}))
		if err != nil {
			return jobsLoadedMsg{err: err}
		}
		return jobsLoadedMsg{jobs: resp.Msg.GetJobs()}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(jobsRefreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func openWatchCmd(ctx context.Context, client jobv1connect.JobServiceClient, jobID string) tea.Cmd {
	return func() tea.Msg {
		stream, err := client.WatchJob(ctx, connect.NewRequest(&jobv1.WatchJobRequest{JobId: jobID}))
		if err != nil {
			return streamOpenedMsg{err: err}
		}
		return streamOpenedMsg{stream: stream}
	}
}

// waitForJobEventCmd blocks on stream.Receive(). This is safe because
// Bubble Tea runs each tea.Cmd in its own goroutine; Update re-issues this
// command after every event to keep listening.
func waitForJobEventCmd(stream jobEventStream) tea.Cmd {
	return func() tea.Msg {
		if stream.Receive() {
			return jobEventMsg{event: stream.Msg()}
		}
		if err := stream.Err(); err != nil {
			return streamErrMsg{err: err}
		}
		return streamClosedMsg{}
	}
}

func (m jobsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case jobsLoadedMsg:
		m.listErr = msg.err
		if msg.err == nil {
			m.jobs = msg.jobs
			m.cursor = clampCursor(m.cursor, len(m.jobs))
		}
		return m, nil
	case tickMsg:
		if m.screen != screenList {
			return m, nil
		}
		return m, tea.Batch(fetchJobsCmd(m.ctx, m.client), tickCmd())
	case streamOpenedMsg:
		m.streamErr = msg.err
		m.stream = msg.stream
		if msg.err != nil {
			return m, nil
		}
		return m, waitForJobEventCmd(m.stream)
	case jobEventMsg:
		if job := msg.event.GetJob(); job != nil {
			m.selected = job
		}
		return m, waitForJobEventCmd(m.stream)
	case streamErrMsg:
		m.streamErr = msg.err
		return m, nil
	case streamClosedMsg:
		m.streamDone = true
		return m, nil
	}
	return m, nil
}

func (m jobsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	switch m.screen {
	case screenDetail:
		return m.handleDetailKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

func (m jobsModel) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.jobs)-1 {
			m.cursor++
		}
	case "r":
		return m, fetchJobsCmd(m.ctx, m.client)
	case "enter":
		if len(m.jobs) == 0 {
			return m, nil
		}
		job := m.jobs[m.cursor]
		m.screen = screenDetail
		m.selected = job
		m.taskCursor = 0
		m.stream = nil
		m.streamErr = nil
		m.streamDone = false
		return m, openWatchCmd(m.ctx, m.client, job.GetId())
	}
	return m, nil
}

func (m jobsModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.taskCursor > 0 {
			m.taskCursor--
		}
	case "down", "j":
		if m.selected != nil && m.taskCursor < len(m.selected.GetTasks())-1 {
			m.taskCursor++
		}
	case "esc", "b":
		if m.stream != nil {
			_ = m.stream.Close()
		}
		m.screen = screenList
		m.stream = nil
		m.selected = nil
		m.streamErr = nil
		m.streamDone = false
		return m, tea.Batch(fetchJobsCmd(m.ctx, m.client), tickCmd())
	}
	return m, nil
}

func clampCursor(cursor, length int) int {
	switch {
	case length == 0:
		return 0
	case cursor >= length:
		return length - 1
	case cursor < 0:
		return 0
	default:
		return cursor
	}
}

func (m jobsModel) View() string {
	if m.screen == screenDetail {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m jobsModel) viewList() string {
	var b strings.Builder
	b.WriteString("Jobs  (↑/↓ move, enter watch, r refresh, q quit)\n\n")
	if m.listErr != nil {
		b.WriteString(pterm.LightRed(fmt.Sprintf("error listing jobs: %v\n\n", m.listErr)))
	}
	if len(m.jobs) == 0 {
		b.WriteString("no jobs yet\n")
		return b.String()
	}
	for i, job := range m.jobs {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		fmt.Fprintf(&b, "%s%-36s %-20s %-11s %s\n", cursor, job.GetId(), job.GetKind(), jobStatusLabel(job.GetStatus()), formatProgress(job.GetProgress()))
	}
	return b.String()
}

func (m jobsModel) viewDetail() string {
	var b strings.Builder
	if m.selected == nil {
		b.WriteString("loading job...\n")
		return b.String()
	}
	job := m.selected
	fmt.Fprintf(&b, "Job %s  kind=%s  status=%s  progress=%s\n", job.GetId(), job.GetKind(), jobStatusLabel(job.GetStatus()), formatProgress(job.GetProgress()))
	b.WriteString("(esc/b: back, ↑/↓: select task, q: quit)\n\n")

	switch {
	case m.streamErr != nil:
		b.WriteString(pterm.LightRed(fmt.Sprintf("stream error: %v\n\n", m.streamErr)))
	case m.streamDone:
		b.WriteString(pterm.Gray("stream closed\n\n"))
	}

	tasks := job.GetTasks()
	if len(tasks) == 0 {
		b.WriteString("no tasks yet\n")
		return b.String()
	}

	b.WriteString("Tasks:\n")
	for i, task := range tasks {
		cursor := "  "
		if i == m.taskCursor {
			cursor = "> "
		}
		fmt.Fprintf(&b, "%s%-40s %-11s %s\n", cursor, task.GetLabel(), jobStatusLabel(task.GetStatus()), formatProgress(task.GetProgress()))
	}

	taskCursor := clampCursor(m.taskCursor, len(tasks))
	selectedTask := tasks[taskCursor]
	fmt.Fprintf(&b, "\nSteps for %s:\n", selectedTask.GetLabel())
	for _, step := range selectedTask.GetSteps() {
		fmt.Fprintf(&b, "  %-24s %-11s %-10s %s\n", step.GetName(), jobStatusLabel(step.GetStatus()), formatDuration(step.GetStartedAt(), step.GetFinishedAt()), step.GetMessage())
	}
	return b.String()
}
