package jobqueue_test

import (
	"context"
	"purser/pkg/jobqueue"
	"purser/pkg/jobqueue/memory"
	"testing"
)

func TestEngine_DiagnosticExecutor(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"track one", "track two", "track three"}, nil)
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

// TestEngine_DiagnosticExecutor_PartialOnOneFailedTask covers the
// aggregation rule from docs/adr/0023-job-queue.md, decided concretely by
// this issue: a job is failed only if every task failed; if at least one
// task succeeded and at least one failed, the job is partial.
func TestEngine_DiagnosticExecutor_PartialOnOneFailedTask(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	params := map[string]string{"fail_at_step:track two": "1"}
	job, err := eng.Trigger(context.Background(), "diagnostic", []string{"track one", "track two", "track three"}, params)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	final := waitForTerminal(t, eng, job.ID)
	if final.Status != jobqueue.StatusPartial {
		t.Fatalf("Job.Status = %q, want %q", final.Status, jobqueue.StatusPartial)
	}

	var failedTask, otherOne, otherTwo *jobqueue.Task
	for _, task := range final.Tasks {
		switch task.Label {
		case "track two":
			failedTask = task
		case "track one":
			otherOne = task
		case "track three":
			otherTwo = task
		}
	}

	if failedTask == nil {
		t.Fatal("failed task not found in job.Tasks")
	}
	if failedTask.Status != jobqueue.StatusFailed {
		t.Fatalf("failed Task.Status = %q, want %q", failedTask.Status, jobqueue.StatusFailed)
	}
	// The configured fail point is step index 1 ("process"): the executor
	// stops the task there rather than running "finalize" too, since the
	// task is already failed.
	if len(failedTask.Steps) != 2 {
		t.Fatalf("failed Task has %d steps, want 2 (stopped at the failing step)", len(failedTask.Steps))
	}
	failedStep := failedTask.Steps[1]
	if failedStep.Status != jobqueue.StatusFailed {
		t.Fatalf("failing Step.Status = %q, want %q", failedStep.Status, jobqueue.StatusFailed)
	}
	if failedStep.Message == "" {
		t.Fatal("failing Step.Message is empty, want the synthetic error text")
	}
	if failedStep.Detail["elapsed_ms"] == "" {
		t.Fatal("failing Step.Detail has no elapsed_ms")
	}

	for _, task := range []*jobqueue.Task{otherOne, otherTwo} {
		if task == nil {
			t.Fatal("an unaffected task is missing from job.Tasks")
		}
		if task.Status != jobqueue.StatusSucceeded {
			t.Fatalf("unaffected Task %q Status = %q, want %q", task.Label, task.Status, jobqueue.StatusSucceeded)
		}
		if len(task.Steps) != 3 {
			t.Fatalf("unaffected Task %q has %d steps, want 3", task.Label, len(task.Steps))
		}
	}
}

// TestEngine_DiagnosticExecutor_FailedWhenEveryTaskFails covers the other
// half of the same aggregation rule: every task failing makes the job
// failed, not partial.
func TestEngine_DiagnosticExecutor_FailedWhenEveryTaskFails(t *testing.T) {
	eng := jobqueue.NewEngine(memory.New(), jobqueue.WithLogger(discardLogger()))

	taskLabels := []string{"track one", "track two", "track three"}
	params := map[string]string{}
	for _, label := range taskLabels {
		params["fail_at_step:"+label] = "0"
	}

	job, err := eng.Trigger(context.Background(), "diagnostic", taskLabels, params)
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}

	final := waitForTerminal(t, eng, job.ID)
	if final.Status != jobqueue.StatusFailed {
		t.Fatalf("Job.Status = %q, want %q", final.Status, jobqueue.StatusFailed)
	}
	for _, task := range final.Tasks {
		if task.Status != jobqueue.StatusFailed {
			t.Fatalf("Task %q Status = %q, want %q", task.Label, task.Status, jobqueue.StatusFailed)
		}
	}
}
