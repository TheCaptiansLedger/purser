package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/gen/go/purser/job/v1/jobv1connect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	jobv1 "purser/gen/go/purser/job/v1"
)

// fakeJobEventStream is a hand-rolled jobEventStream, standing in for a
// live WatchJob RPC's *connect.ServerStreamForClient. This is the seam
// docs/adr/0002-solid-design-principles.md's DIP/ISP guidance describes:
// the model depends on this narrow interface, not the concrete stream type.
type fakeJobEventStream struct {
	events []*jobv1.JobEvent
	pos    int
	err    error
	closed bool
}

func (f *fakeJobEventStream) Receive() bool {
	if f.pos >= len(f.events) {
		return false
	}
	f.pos++
	return true
}

func (f *fakeJobEventStream) Msg() *jobv1.JobEvent {
	return f.events[f.pos-1]
}

func (f *fakeJobEventStream) Err() error {
	return f.err
}

func (f *fakeJobEventStream) Close() error {
	f.closed = true
	return nil
}

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		length int
		want   int
	}{
		{"empty list", 3, 0, 0},
		{"in bounds", 2, 5, 2},
		{"past end", 9, 5, 4},
		{"negative", -1, 5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampCursor(tt.cursor, tt.length); got != tt.want {
				t.Errorf("clampCursor(%d, %d) = %d, want %d", tt.cursor, tt.length, got, tt.want)
			}
		})
	}
}

func TestIndexOfJob(t *testing.T) {
	jobs := []*jobv1.Job{{Id: "a"}, {Id: "b"}, {Id: "c"}}

	if got := indexOfJob(jobs, "b"); got != 1 {
		t.Errorf("indexOfJob(b) = %d, want 1", got)
	}
	if got := indexOfJob(jobs, "missing"); got != -1 {
		t.Errorf("indexOfJob(missing) = %d, want -1", got)
	}
	if got := indexOfJob(nil, "a"); got != -1 {
		t.Errorf("indexOfJob(nil, a) = %d, want -1", got)
	}
}

func TestEnsureRowVisible(t *testing.T) {
	vp := viewport.New(10, 5)
	vp.SetContent(strings.Repeat("line\n", 20))

	ensureRowVisible(&vp, 3)
	if vp.YOffset != 0 {
		t.Fatalf("row already in view should not scroll, offset = %d", vp.YOffset)
	}

	ensureRowVisible(&vp, 10)
	if vp.YOffset != 6 {
		t.Fatalf("offset after scrolling to row 10 = %d, want 6", vp.YOffset)
	}

	ensureRowVisible(&vp, 0)
	if vp.YOffset != 0 {
		t.Fatalf("offset after scrolling back to row 0 = %d, want 0", vp.YOffset)
	}
}

func TestWaitForJobEventCmd(t *testing.T) {
	t.Run("event received", func(t *testing.T) {
		event := &jobv1.JobEvent{Job: &jobv1.Job{Id: "job-1"}}
		stream := &fakeJobEventStream{events: []*jobv1.JobEvent{event}}

		msg := waitForJobEventCmd(stream, 5)()

		got, ok := msg.(jobEventMsg)
		if !ok {
			t.Fatalf("got %T, want jobEventMsg", msg)
		}
		if got.gen != 5 {
			t.Errorf("gen = %d, want 5", got.gen)
		}
		if got.event.GetJob().GetId() != "job-1" {
			t.Errorf("event job id = %q, want %q", got.event.GetJob().GetId(), "job-1")
		}
	})

	t.Run("stream error", func(t *testing.T) {
		wantErr := errors.New("boom")
		stream := &fakeJobEventStream{err: wantErr}

		msg := waitForJobEventCmd(stream, 5)()

		got, ok := msg.(streamErrMsg)
		if !ok {
			t.Fatalf("got %T, want streamErrMsg", msg)
		}
		if got.gen != 5 {
			t.Errorf("gen = %d, want 5", got.gen)
		}
		if !errors.Is(got.err, wantErr) {
			t.Errorf("err = %v, want %v", got.err, wantErr)
		}
	})

	t.Run("stream closed cleanly", func(t *testing.T) {
		stream := &fakeJobEventStream{}

		msg := waitForJobEventCmd(stream, 5)()

		got, ok := msg.(streamClosedMsg)
		if !ok {
			t.Fatalf("got %T, want streamClosedMsg", msg)
		}
		if got.gen != 5 {
			t.Errorf("gen = %d, want 5", got.gen)
		}
	})
}

func TestJobsModelUpdate_ListCursorBounds(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.jobs = []*jobv1.Job{{Id: "a"}, {Id: "b"}, {Id: "c"}}
	m.selectedID = "a" // pretend "a" is already watched, so bound-checks don't churn the stream

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(jobsModel)
	if m.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", m.cursor)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(jobsModel)
	if m.cursor != 0 {
		t.Fatalf("cursor after up = %d, want 0", m.cursor)
	}

	// Up at the top stays put.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(jobsModel)
	if m.cursor != 0 {
		t.Fatalf("cursor after up-at-top = %d, want 0", m.cursor)
	}
	if cmd != nil {
		t.Fatal("expected no watch-stream reopen when the cursor doesn't move off the watched job")
	}

	m.cursor = 2
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(jobsModel)
	if m.cursor != 2 {
		t.Fatalf("cursor past the end = %d, want 2", m.cursor)
	}
}

func TestJobsModelUpdate_CursorMoveOpensWatchForNewSelection(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.jobs = []*jobv1.Job{{Id: "job-1"}, {Id: "job-2"}}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(jobsModel)

	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	if m.selectedID != "job-2" {
		t.Fatalf("selectedID = %q, want %q", m.selectedID, "job-2")
	}
	if m.selected.GetId() != "job-2" {
		t.Fatalf("selected id = %q, want %q", m.selected.GetId(), "job-2")
	}
	if m.streamGen != 1 {
		t.Fatalf("streamGen = %d, want 1", m.streamGen)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil cmd to open the watch stream for the newly highlighted job")
	}
}

func TestJobsModelUpdate_CursorMoveClosesPreviousStream(t *testing.T) {
	stream := &fakeJobEventStream{}
	m := newJobsModel(context.Background(), nil)
	m.jobs = []*jobv1.Job{{Id: "job-1"}, {Id: "job-2"}}
	m.selectedID = "job-1"
	m.selected = &jobv1.Job{Id: "job-1"}
	m.stream = stream
	m.streamGen = 1

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(jobsModel)

	if !stream.closed {
		t.Fatal("expected the previously watched stream to be closed")
	}
	if m.stream != nil {
		t.Fatal("expected the stream field to be cleared until the new one opens")
	}
	if m.selectedID != "job-2" {
		t.Fatalf("selectedID = %q, want %q", m.selectedID, "job-2")
	}
	if m.streamGen != 2 {
		t.Fatalf("streamGen = %d, want 2", m.streamGen)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil cmd to open the watch stream for the newly highlighted job")
	}
}

func TestJobsModelUpdate_StaleStreamMessagesAreIgnored(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.streamGen = 2
	m.selected = &jobv1.Job{Id: "job-1", Status: jobv1.JobStatus_JOB_STATUS_RUNNING}

	stale := &jobv1.Job{Id: "job-1", Status: jobv1.JobStatus_JOB_STATUS_SUCCEEDED}
	next, cmd := m.Update(jobEventMsg{gen: 1, event: &jobv1.JobEvent{Job: stale}})
	m = next.(jobsModel)
	if m.selected.GetStatus() != jobv1.JobStatus_JOB_STATUS_RUNNING {
		t.Fatalf("stale jobEventMsg should be dropped, status = %v", m.selected.GetStatus())
	}
	if cmd != nil {
		t.Fatal("expected no follow-up cmd for a stale jobEventMsg")
	}

	next, cmd = m.Update(streamErrMsg{gen: 1, err: errors.New("stale error")})
	m = next.(jobsModel)
	if m.streamErr != nil {
		t.Fatalf("stale streamErrMsg should be dropped, streamErr = %v", m.streamErr)
	}
	if cmd != nil {
		t.Fatal("expected no cmd for a stale streamErrMsg")
	}

	next, _ = m.Update(streamClosedMsg{gen: 1})
	m = next.(jobsModel)
	if m.streamDone {
		t.Fatal("stale streamClosedMsg should be dropped")
	}

	// A matching-gen message does apply.
	current := &jobv1.Job{Id: "job-1", Status: jobv1.JobStatus_JOB_STATUS_SUCCEEDED}
	next, cmd = m.Update(jobEventMsg{gen: 2, event: &jobv1.JobEvent{Job: current}})
	m = next.(jobsModel)
	if m.selected.GetStatus() != jobv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("current-gen jobEventMsg should apply, status = %v", m.selected.GetStatus())
	}
	if cmd == nil {
		t.Fatal("expected Update to re-issue waitForJobEventCmd for a current-gen event")
	}
}

func TestJobsModelUpdate_StreamOpened(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.streamGen = 3

	stream := &fakeJobEventStream{}
	next, cmd := m.Update(streamOpenedMsg{gen: 3, stream: stream})
	m = next.(jobsModel)
	if m.stream != stream {
		t.Fatal("expected the opened stream to be stored on the model")
	}
	if cmd == nil {
		t.Fatal("expected Update to start waiting on the newly opened stream")
	}

	wantErr := errors.New("open failed")
	next, cmd = m.Update(streamOpenedMsg{gen: 3, err: wantErr})
	m = next.(jobsModel)
	if !errors.Is(m.streamErr, wantErr) {
		t.Fatalf("streamErr = %v, want %v", m.streamErr, wantErr)
	}
	if cmd != nil {
		t.Fatal("expected no follow-up cmd when opening the stream failed")
	}

	// A stale-gen open response is dropped entirely.
	next, cmd = m.Update(streamOpenedMsg{gen: 1, stream: &fakeJobEventStream{}})
	m2 := next.(jobsModel)
	if m2.stream != m.stream {
		t.Fatal("expected a stale-gen streamOpenedMsg to be ignored")
	}
	if cmd != nil {
		t.Fatal("expected no cmd for a stale-gen streamOpenedMsg")
	}
}

func TestJobsModelUpdate_TabTogglesFocus(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	if m.focus != paneList {
		t.Fatalf("initial focus = %v, want paneList", m.focus)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(jobsModel)
	if m.focus != paneDetail {
		t.Fatalf("focus after tab = %v, want paneDetail", m.focus)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(jobsModel)
	if m.focus != paneList {
		t.Fatalf("focus after second tab = %v, want paneList", m.focus)
	}
}

func TestJobsModelUpdate_DetailFocusTaskNavigation(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.focus = paneDetail
	m.selected = &jobv1.Job{
		Id: "job-1",
		Tasks: []*jobv1.Task{
			{Id: "t1", Label: "track 1"},
			{Id: "t2", Label: "track 2"},
		},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(jobsModel)
	if m.taskCursor != 1 {
		t.Fatalf("taskCursor after down = %d, want 1", m.taskCursor)
	}

	// Down at the last task stays put.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(jobsModel)
	if m.taskCursor != 1 {
		t.Fatalf("taskCursor past the end = %d, want 1", m.taskCursor)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(jobsModel)
	if m.taskCursor != 0 {
		t.Fatalf("taskCursor after up = %d, want 0", m.taskCursor)
	}
}

func TestJobsModelUpdate_QuitKeysCloseStream(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyCtrlC}, {Type: tea.KeyRunes, Runes: []rune("q")}} {
		stream := &fakeJobEventStream{}
		m := newJobsModel(context.Background(), nil)
		m.stream = stream
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("key %v: expected a quit cmd", key)
		}
		if !stream.closed {
			t.Fatalf("key %v: expected the open stream to be closed on quit", key)
		}
	}
}

func TestJobsModelUpdate_JobsLoadedSelectsFirstJobOnInitialLoad(t *testing.T) {
	m := newJobsModel(context.Background(), nil)

	jobs := []*jobv1.Job{{Id: "job-1", Kind: "diagnostic"}, {Id: "job-2"}}
	next, cmd := m.Update(jobsLoadedMsg{jobs: jobs})
	m = next.(jobsModel)

	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}
	if m.selectedID != "job-1" {
		t.Fatalf("selectedID = %q, want %q", m.selectedID, "job-1")
	}
	if cmd == nil {
		t.Fatal("expected a non-nil cmd to open the watch stream for the first job")
	}
}

func TestJobsModelUpdate_JobsLoadedTracksSelectedJobAcrossReorder(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.jobs = []*jobv1.Job{{Id: "job-1"}, {Id: "job-2"}}
	m.cursor = 1
	m.selectedID = "job-2"
	m.selected = &jobv1.Job{Id: "job-2", Status: jobv1.JobStatus_JOB_STATUS_RUNNING}
	m.streamGen = 1

	reordered := []*jobv1.Job{{Id: "job-2"}, {Id: "job-1"}}
	next, cmd := m.Update(jobsLoadedMsg{jobs: reordered})
	m = next.(jobsModel)

	if m.cursor != 0 {
		t.Fatalf("cursor should follow the watched job to its new index, got %d, want 0", m.cursor)
	}
	if m.selectedID != "job-2" {
		t.Fatalf("selectedID = %q, want %q", m.selectedID, "job-2")
	}
	if m.streamGen != 1 {
		t.Fatalf("streamGen = %d, want 1 (no reopen expected)", m.streamGen)
	}
	if cmd != nil {
		t.Fatal("expected no watch-stream reopen when the watched job merely changed position")
	}
}

func TestJobsModelUpdate_JobsLoadedErr(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.jobs = []*jobv1.Job{{Id: "a"}}

	wantErr := errors.New("list failed")
	next, _ := m.Update(jobsLoadedMsg{err: wantErr})
	m = next.(jobsModel)

	if !errors.Is(m.listErr, wantErr) {
		t.Fatalf("listErr = %v, want %v", m.listErr, wantErr)
	}
	if len(m.jobs) != 1 {
		t.Fatalf("jobs should be left untouched on a failed refresh, got %d", len(m.jobs))
	}
}

func TestJobsModelUpdate_TickAlwaysRefreshes(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	m.focus = paneDetail

	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected tick to refresh and reschedule regardless of pane focus")
	}
}

func TestJobsModelUpdate_RRefreshesList(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Fatal("expected r to trigger a refresh")
	}
}

func TestJobsModelUpdate_WindowSizeMsgSizesViewports(t *testing.T) {
	m := newJobsModel(context.Background(), nil)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(jobsModel)

	if !m.ready {
		t.Fatal("expected ready to be true after a WindowSizeMsg")
	}
	if m.listViewport.Width <= 0 || m.listViewport.Height <= 0 {
		t.Fatalf("list viewport not sized: width=%d height=%d", m.listViewport.Width, m.listViewport.Height)
	}
	if m.detailViewport.Width <= 0 || m.detailViewport.Height <= 0 {
		t.Fatalf("detail viewport not sized: width=%d height=%d", m.detailViewport.Width, m.detailViewport.Height)
	}
}

func TestFetchJobsCmd_And_OpenWatchCmd_Errors(t *testing.T) {
	// An address nothing is listening on forces both RPCs down their error
	// paths.
	client := jobv1connect.NewJobServiceClient(http.DefaultClient, "http://127.0.0.1:1")

	msg := fetchJobsCmd(context.Background(), client)()
	loaded, ok := msg.(jobsLoadedMsg)
	if !ok {
		t.Fatalf("got %T, want jobsLoadedMsg", msg)
	}
	if loaded.err == nil {
		t.Fatal("expected fetchJobsCmd to surface a connection error")
	}

	// WatchJob's connection error, if any, may only surface once the
	// stream is actually read rather than at open time (connect-go
	// negotiates the stream lazily) — either openWatchCmd itself fails,
	// or the subsequent Receive() does.
	watchMsg := openWatchCmd(context.Background(), client, "job-1", 1)()
	opened, ok := watchMsg.(streamOpenedMsg)
	if !ok {
		t.Fatalf("got %T, want streamOpenedMsg", watchMsg)
	}
	if opened.err != nil {
		return
	}
	eventMsg := waitForJobEventCmd(opened.stream, 1)()
	if _, ok := eventMsg.(streamErrMsg); !ok {
		t.Fatalf("got %T, want streamErrMsg once the stream is read", eventMsg)
	}
}

func TestJobsModelInit(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected Init to return a non-nil batch cmd")
	}
}

func TestJobsModelView(t *testing.T) {
	m := newJobsModel(context.Background(), nil)
	if got := m.View(); got != "loading..." {
		t.Fatalf("expected the loading placeholder before the first WindowSizeMsg, got %q", got)
	}

	m.width, m.height = 120, 40
	m.ready = true
	m.applySize()

	if got := m.View(); got == "" {
		t.Fatal("expected non-empty view for an empty job list")
	}

	m.jobs = []*jobv1.Job{{Id: "job-1", Kind: "diagnostic", Status: jobv1.JobStatus_JOB_STATUS_RUNNING}}
	m.updateListViewportContent()
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty list view with jobs")
	}
	m.listErr = errors.New("boom")
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty list view with an error")
	}
	m.listErr = nil

	m.selected = &jobv1.Job{
		Id:     "job-1",
		Status: jobv1.JobStatus_JOB_STATUS_RUNNING,
		Tasks: []*jobv1.Task{
			{Id: "t1", Label: "track 1", Steps: []*jobv1.Step{
				{Name: "hash", Status: jobv1.JobStatus_JOB_STATUS_SUCCEEDED, Message: "ok"},
			}},
		},
	}
	m.updateDetailViewportContent()
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty detail view with tasks/steps")
	}

	m.streamErr = errors.New("stream broke")
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty detail view with a stream error")
	}
	m.streamErr = nil
	m.streamDone = true
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty detail view when the stream closed")
	}

	m.focus = paneDetail
	if got := m.View(); got == "" {
		t.Fatal("expected non-empty view with the detail pane focused")
	}
}

// TestFetchAndWatchAgainstRealHandler exercises fetchJobsCmd and
// openWatchCmd against a real jobv1connect handler over HTTP, since
// WatchJob's client return type (*connect.ServerStreamForClient) can't be
// hand-faked the way jobEventStream can — this is the one place these
// tests need a live Connect server instead of a substitutable interface.
func TestFetchAndWatchAgainstRealHandler(t *testing.T) {
	mux := http.NewServeMux()
	path, handler := jobv1connect.NewJobServiceHandler(&fakeJobServiceHandler{
		job: &jobv1.Job{Id: "job-1", Kind: "diagnostic", Status: jobv1.JobStatus_JOB_STATUS_RUNNING},
	})
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := jobv1connect.NewJobServiceClient(srv.Client(), srv.URL)
	ctx := context.Background()

	msg := fetchJobsCmd(ctx, client)()
	loaded, ok := msg.(jobsLoadedMsg)
	if !ok {
		t.Fatalf("got %T, want jobsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected error: %v", loaded.err)
	}
	if len(loaded.jobs) != 1 || loaded.jobs[0].GetId() != "job-1" {
		t.Fatalf("jobs = %+v, want one job with id job-1", loaded.jobs)
	}

	watchMsg := openWatchCmd(ctx, client, "job-1", 1)()
	opened, ok := watchMsg.(streamOpenedMsg)
	if !ok {
		t.Fatalf("got %T, want streamOpenedMsg", watchMsg)
	}
	if opened.err != nil {
		t.Fatalf("unexpected error opening stream: %v", opened.err)
	}
	defer opened.stream.Close()

	eventMsg := waitForJobEventCmd(opened.stream, 1)()
	event, ok := eventMsg.(jobEventMsg)
	if !ok {
		t.Fatalf("got %T, want jobEventMsg", eventMsg)
	}
	if event.event.GetJob().GetId() != "job-1" {
		t.Fatalf("streamed job id = %q, want %q", event.event.GetJob().GetId(), "job-1")
	}
}

type fakeJobServiceHandler struct {
	jobv1connect.UnimplementedJobServiceHandler
	job *jobv1.Job
}

func (f *fakeJobServiceHandler) ListJobs(_ context.Context, _ *connect.Request[jobv1.ListJobsRequest]) (*connect.Response[jobv1.ListJobsResponse], error) {
	return connect.NewResponse(&jobv1.ListJobsResponse{Jobs: []*jobv1.Job{f.job}}), nil
}

func (f *fakeJobServiceHandler) WatchJob(_ context.Context, _ *connect.Request[jobv1.WatchJobRequest], stream *connect.ServerStream[jobv1.JobEvent]) error {
	return stream.Send(&jobv1.JobEvent{Kind: jobv1.JobEventKind_JOB_EVENT_KIND_JOB, Job: f.job})
}
