package jobqueue_test

import (
	"context"
	"purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"testing"
)

func TestEngine_DiagnosticExecutor(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"track one", "track two", "track three"})
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	final := waitForTerminal(t, eng, job.ID)

	if final.Status != jobqueue.StatusSucceeded {
		t.Fatalf("Job.Status = %q, want %q", final.Status, jobqueue.StatusSucceeded)
	}
	if final.Progress() != 1 {
		t.Fatalf("Job.Progress() = %v, want 1", final.Progress())
	}
	if len(final.Tasks) != 3 {
		t.Fatalf("len(Tasks) = %d, want 3", len(final.Tasks))
	}

	for _, task := range final.Tasks {
		if task.Status != jobqueue.StatusSucceeded {
			t.Fatalf("Task %q Status = %q, want %q", task.Label, task.Status, jobqueue.StatusSucceeded)
		}
		if task.Progress() != 1 {
			t.Fatalf("Task %q Progress() = %v, want 1", task.Label, task.Progress())
		}
		if len(task.Steps) != 3 {
			t.Fatalf("Task %q has %d steps, want 3", task.Label, len(task.Steps))
		}
		for _, step := range task.Steps {
			if step.Status != jobqueue.StatusSucceeded {
				t.Fatalf("Step %q Status = %q, want %q", step.Name, step.Status, jobqueue.StatusSucceeded)
			}
			if step.StartedAt.IsZero() || step.FinishedAt.IsZero() {
				t.Fatalf("Step %q has zero StartedAt/FinishedAt", step.Name)
			}
			if step.Detail["elapsed_ms"] == "" {
				t.Fatalf("Step %q has no elapsed_ms Detail", step.Name)
			}
		}
	}
}
