package jobqueue_test

import (
	"context"
	"log/slog"
	"purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"sync"
	"testing"
)

// captureHandler is a minimal slog.Handler that records every Record it
// receives, so a test can assert on level and structured attributes
// rather than parsing formatted log text.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *captureHandler) find(message string, level slog.Level) []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []slog.Record
	for _, r := range h.records {
		if r.Message == message && r.Level == level {
			out = append(out, r)
		}
	}
	return out
}

// TestRunner_FinishStep_FailureLogsAtErrorLevelWithErrorAttribute covers
// docs/adr/0008-structured-logging.md's rule that errors are values, not
// string concatenation: a failing Step's "step finished" log line must be
// Error-level and carry the error as a structured "error" attribute
// (slog.Any), not merely a formatted message.
func TestRunner_FinishStep_FailureLogsAtErrorLevelWithErrorAttribute(t *testing.T) {
	capture := &captureHandler{}
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(slog.New(capture)))

	params := map[string]string{"fail_at_step:track one": "0"}
	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"track one"}, params)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	waitForTerminal(t, eng, job.ID)

	errorRecords := capture.find("step finished", slog.LevelError)
	if len(errorRecords) != 1 {
		t.Fatalf("got %d error-level %q records, want 1", len(errorRecords), "step finished")
	}

	found := false
	errorRecords[0].Attrs(func(a slog.Attr) bool {
		if a.Key != "error" {
			return true
		}
		found = true
		if a.Value.Any() == nil {
			t.Fatal("error attribute has a nil value")
		}
		if _, ok := a.Value.Any().(error); !ok {
			t.Fatalf("error attribute value is %T, want error", a.Value.Any())
		}
		return true
	})
	if !found {
		t.Fatal("error-level \"step finished\" record has no \"error\" attribute")
	}

	infoRecords := capture.find("step finished", slog.LevelInfo)
	if len(infoRecords) != 0 {
		t.Fatalf("got %d info-level %q records for a job with only a failing step, want 0", len(infoRecords), "step finished")
	}
}
