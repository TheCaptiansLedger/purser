package fsnotify_test

import (
	"log/slog"
	"purser/pkg/fswatch"
	"purser/pkg/fswatch/fsnotify"
	"purser/pkg/fswatch/fswatchtest"
	"testing"
)

func TestWatcher_SourceContract(t *testing.T) {
	fswatchtest.TestSource(t, func(t *testing.T) fswatch.Source {
		w, err := fsnotify.New()
		if err != nil {
			t.Fatalf("fsnotify.New: %v", err)
		}
		t.Cleanup(func() {
			if err := w.Close(); err != nil {
				t.Errorf("Close returned error: %v", err)
			}
		})
		return w
	})
}

func TestNew_WithLoggerAndErrorsChannel(t *testing.T) {
	w, err := fsnotify.New(fsnotify.WithLogger(slog.Default()))
	if err != nil {
		t.Fatalf("fsnotify.New: %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}()

	if w.Errors() == nil {
		t.Fatal("Errors() returned a nil channel")
	}
}
