package main

import (
	"context"
	"fmt"
	"purser/gen/go/purser/job/v1/jobv1connect"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// pane identifies which of the two split panes has keyboard focus.
type pane int

const (
	paneList pane = iota
	paneDetail
)

const (
	jobsRefreshInterval = 3 * time.Second
	jobsListPageSize    = 50
)

// jobsModel is the Bubble Tea model backing `purser jobs`. It renders a
// single split-pane screen: a polled, scrollable list of recent jobs on the
// left, and a live-streamed (WatchJob) detail view of tasks/steps for
// whichever job is currently highlighted on the right. The watched job
// follows the list highlight automatically — moving the cursor swaps the
// stream, no separate "open detail" step is needed.
type jobsModel struct {
	ctx    context.Context
	client jobv1connect.JobServiceClient
	addr   string

	width  int
	height int
	ready  bool

	focus pane

	jobs    []*jobv1.Job
	cursor  int
	listErr error

	listViewport viewport.Model

	// selectedID is the id of the job the detail pane is currently
	// watching. It's tracked separately from m.jobs[m.cursor] so a
	// background list refresh that reshuffles job order doesn't spuriously
	// look like a highlight change and restart the stream.
	selectedID string
	selected   *jobv1.Job

	stream     jobEventStream
	streamGen  int // incremented on every selectJob; guards stale async stream messages
	taskCursor int
	streamErr  error
	streamDone bool

	detailViewport viewport.Model
}

func newJobsModel(ctx context.Context, client jobv1connect.JobServiceClient) jobsModel {
	return jobsModel{
		ctx:            ctx,
		client:         client,
		focus:          paneList,
		listViewport:   viewport.New(0, 0),
		detailViewport: viewport.New(0, 0),
	}
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
	gen    int
	stream jobEventStream
	err    error
}

type jobEventMsg struct {
	gen   int
	event *jobv1.JobEvent
}

type streamErrMsg struct {
	gen int
	err error
}

type streamClosedMsg struct {
	gen int
}

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

func openWatchCmd(ctx context.Context, client jobv1connect.JobServiceClient, jobID string, gen int) tea.Cmd {
	return func() tea.Msg {
		stream, err := client.WatchJob(ctx, connect.NewRequest(&jobv1.WatchJobRequest{JobId: jobID}))
		if err != nil {
			return streamOpenedMsg{gen: gen, err: err}
		}
		return streamOpenedMsg{gen: gen, stream: stream}
	}
}

// waitForJobEventCmd blocks on stream.Receive(). This is safe because
// Bubble Tea runs each tea.Cmd in its own goroutine; Update re-issues this
// command after every event to keep listening. gen ties the resulting
// message back to the stream generation it came from, so Update can drop
// events from a stream that's since been superseded by a highlight change.
func waitForJobEventCmd(stream jobEventStream, gen int) tea.Cmd {
	return func() tea.Msg {
		if stream.Receive() {
			return jobEventMsg{gen: gen, event: stream.Msg()}
		}
		if err := stream.Err(); err != nil {
			return streamErrMsg{gen: gen, err: err}
		}
		return streamClosedMsg{gen: gen}
	}
}

func (m jobsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.applySize()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case jobsLoadedMsg:
		return m.handleJobsLoaded(msg)
	case tickMsg:
		return m, tea.Batch(fetchJobsCmd(m.ctx, m.client), tickCmd())
	case streamOpenedMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		m.streamErr = msg.err
		m.stream = msg.stream
		if msg.err != nil {
			return m, nil
		}
		return m, waitForJobEventCmd(m.stream, m.streamGen)
	case jobEventMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		if job := msg.event.GetJob(); job != nil {
			m.selected = job
			m.taskCursor = clampCursor(m.taskCursor, len(job.GetTasks()))
			m.updateDetailViewportContent()
		}
		return m, waitForJobEventCmd(m.stream, m.streamGen)
	case streamErrMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		m.streamErr = msg.err
		return m, nil
	case streamClosedMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		m.streamDone = true
		return m, nil
	}
	return m, nil
}

func (m jobsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.stream != nil {
			_ = m.stream.Close()
		}
		return m, tea.Quit
	case "tab":
		if m.focus == paneList {
			m.focus = paneDetail
		} else {
			m.focus = paneList
		}
		return m, nil
	}

	switch m.focus {
	case paneDetail:
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
	default:
		var cmd tea.Cmd
		m.listViewport, cmd = m.listViewport.Update(msg)
		return m, cmd
	}
	m.updateListViewportContent()
	return m.syncSelection()
}

func (m jobsModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.taskCursor > 0 {
			m.taskCursor--
		}
		m.updateDetailViewportContent()
		return m, nil
	case "down", "j":
		if m.selected != nil && m.taskCursor < len(m.selected.GetTasks())-1 {
			m.taskCursor++
		}
		m.updateDetailViewportContent()
		return m, nil
	default:
		var cmd tea.Cmd
		m.detailViewport, cmd = m.detailViewport.Update(msg)
		return m, cmd
	}
}

func (m jobsModel) handleJobsLoaded(msg jobsLoadedMsg) (tea.Model, tea.Cmd) {
	m.listErr = msg.err
	if msg.err != nil {
		return m, nil
	}
	m.jobs = msg.jobs
	if idx := indexOfJob(m.jobs, m.selectedID); m.selectedID != "" && idx >= 0 {
		m.cursor = idx
	} else {
		m.cursor = clampCursor(m.cursor, len(m.jobs))
	}
	m.updateListViewportContent()
	return m.syncSelection()
}

// syncSelection opens a watch stream for whatever job is now highlighted,
// if it differs from the job already being watched.
func (m jobsModel) syncSelection() (jobsModel, tea.Cmd) {
	if len(m.jobs) == 0 {
		return m, nil
	}
	job := m.jobs[m.cursor]
	if job.GetId() == m.selectedID {
		return m, nil
	}
	return m.selectJob(job)
}

// selectJob closes any stream currently open, then opens a new WatchJob
// stream for job. streamGen is bumped so in-flight messages from the old
// stream are recognized as stale and dropped by Update.
func (m jobsModel) selectJob(job *jobv1.Job) (jobsModel, tea.Cmd) {
	if m.stream != nil {
		_ = m.stream.Close()
	}
	m.stream = nil
	m.streamErr = nil
	m.streamDone = false
	m.taskCursor = 0
	m.selectedID = job.GetId()
	m.selected = job
	m.streamGen++
	m.updateDetailViewportContent()
	return m, openWatchCmd(m.ctx, m.client, job.GetId(), m.streamGen)
}

func indexOfJob(jobs []*jobv1.Job, id string) int {
	for i, j := range jobs {
		if j.GetId() == id {
			return i
		}
	}
	return -1
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

// applySize recomputes both viewports' dimensions from the current
// terminal size and re-renders their content at the new size. The list and
// detail panes are stacked vertically, each spanning the full terminal
// width, so neither pane's columns get squeezed into a narrow half-screen.
func (m *jobsModel) applySize() {
	w := m.width - 2 // left/right border
	if w < 1 {
		w = 1
	}
	m.listViewport.Width = w
	m.detailViewport.Width = w

	bodyHeight := m.height - jobsHeaderHeight - jobsFooterHeight
	listOuter, detailOuter := splitPaneHeights(bodyHeight)
	m.listViewport.Height = paneViewportHeight(listOuter)
	m.detailViewport.Height = paneViewportHeight(detailOuter)

	m.updateListViewportContent()
	m.updateDetailViewportContent()
}

// updateListViewportContent re-renders the job table into the list
// viewport and scrolls it to keep the highlighted row visible.
func (m *jobsModel) updateListViewportContent() {
	rows := make([]string, 0, len(m.jobs)+1)
	rows = append(rows, renderJobListHeaderRow())
	if len(m.jobs) == 0 {
		rows = append(rows, "no jobs yet")
	}
	for i, job := range m.jobs {
		rows = append(rows, renderJobRow(job, i == m.cursor))
	}
	m.listViewport.SetContent(strings.Join(rows, "\n"))
	ensureRowVisible(&m.listViewport, m.cursor+1)
}

// updateDetailViewportContent re-renders the highlighted job's tasks/steps
// into the detail viewport and scrolls it to keep the selected task
// visible.
func (m *jobsModel) updateDetailViewportContent() {
	if m.selected == nil {
		m.detailViewport.SetContent("loading job...")
		return
	}
	job := m.selected
	rows := []string{
		fmt.Sprintf("kind: %s   status: %s   progress: %s", job.GetKind(), jobStatusLabel(job.GetStatus()), formatProgress(job.GetProgress())),
		"",
	}

	tasks := job.GetTasks()
	taskCursorRow := -1
	if len(tasks) == 0 {
		rows = append(rows, "no tasks yet")
	} else {
		rows = append(rows, "Tasks:")
		for i, task := range tasks {
			if i == m.taskCursor {
				taskCursorRow = len(rows)
			}
			rows = append(rows, renderTaskRow(task, i == m.taskCursor))
		}
		taskCursor := clampCursor(m.taskCursor, len(tasks))
		selectedTask := tasks[taskCursor]
		rows = append(rows, "", fmt.Sprintf("Steps for %s:", selectedTask.GetLabel()))
		for _, step := range selectedTask.GetSteps() {
			rows = append(rows, renderStepRow(step))
		}
	}
	m.detailViewport.SetContent(strings.Join(rows, "\n"))
	if taskCursorRow >= 0 {
		ensureRowVisible(&m.detailViewport, taskCursorRow)
	}
}

// ensureRowVisible scrolls vp just enough to bring row into view.
func ensureRowVisible(vp *viewport.Model, row int) {
	if vp.Height <= 0 {
		return
	}
	switch {
	case row < vp.YOffset:
		vp.SetYOffset(row)
	case row >= vp.YOffset+vp.Height:
		vp.SetYOffset(row - vp.Height + 1)
	}
}

func (m jobsModel) View() string {
	if !m.ready {
		return "loading..."
	}

	header := renderHeader(m.addr, m.listErr)
	footer := renderFooter(m.focus, m.streamErr, m.streamDone)

	listTitle := fmt.Sprintf("JOBS (%d)", len(m.jobs))
	detailTitle := "DETAIL"
	if m.selected != nil {
		detailTitle = fmt.Sprintf("DETAIL — %s", m.selected.GetId())
	}

	listInterior := m.listViewport.Height + jobsPaneTitleRows
	detailInterior := m.detailViewport.Height + jobsPaneTitleRows

	listPane := renderPane(listTitle, m.listViewport.View(), m.listViewport.Width, listInterior, m.focus == paneList)
	detailPane := renderPane(detailTitle, m.detailViewport.View(), m.detailViewport.Width, detailInterior, m.focus == paneDetail)

	return lipgloss.JoinVertical(lipgloss.Left, header, listPane, detailPane, footer)
}
