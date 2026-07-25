package jobqueue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// diagnosticStepDelay is the brief, real delay each synthetic step sleeps
// for, so Detail's elapsed_ms reflects genuine elapsed time rather than a
// hardcoded value.
const diagnosticStepDelay = 50 * time.Millisecond

// diagnosticSteps are the synthetic steps every Task of a "diagnostic" Job
// runs through, in order.
var diagnosticSteps = []string{"initialize", "process", "finalize"}

// failAtStepParamPrefix is the Job.Params key prefix the diagnostic
// executor looks for: a key "fail_at_step:<task label>" whose value parses
// as a valid index into diagnosticSteps makes that task's step at that
// index fail with a synthetic error, while every other step/task proceeds
// normally. This format is owned entirely by this file — pkg/jobqueue's
// engine passes Params through unexamined. See docs/adr/0023-job-queue.md.
const failAtStepParamPrefix = "fail_at_step:"

// diagnosticFailPoints extracts, from a Job's Params, which task labels
// should fail and at which diagnosticSteps index. A malformed entry
// (non-integer or out-of-range value) is ignored, as if it weren't present.
func diagnosticFailPoints(params map[string]string) map[string]int {
	if len(params) == 0 {
		return nil
	}
	points := make(map[string]int, len(params))
	for key, value := range params {
		label, ok := strings.CutPrefix(key, failAtStepParamPrefix)
		if !ok {
			continue
		}
		idx, err := strconv.Atoi(value)
		if err != nil || idx < 0 || idx >= len(diagnosticSteps) {
			continue
		}
		points[label] = idx
	}
	return points
}

// diagnosticExecutor is the built-in Executor registered under the
// "diagnostic" kind — a permanent smoke-testing capability of the engine
// (not scaffolding), exercising the real Runner API end-to-end against
// every Task/Step it's given, including the failure/partial-status path
// via diagnosticFailPoints. See docs/adr/0023-job-queue.md.
type diagnosticExecutor struct{}

func newDiagnosticExecutor() *diagnosticExecutor {
	return &diagnosticExecutor{}
}

// Execute implements Executor. A configured fail point fails its Task —
// via a failed Step, per docs/adr/0023-job-queue.md's Step-failure ->
// Task-failure rule — but Execute itself always returns nil: task-level
// outcomes are expressed through Task.Status, so the Engine's finalStatus
// aggregates the Job's terminal Status (succeeded/partial/failed) from
// Task outcomes rather than from an executor error.
func (d *diagnosticExecutor) Execute(ctx context.Context, r *Runner) error {
	job, err := r.Job(ctx)
	if err != nil {
		return err
	}

	failPoints := diagnosticFailPoints(job.Params)

	for _, task := range job.Tasks {
		if err := r.StartTask(ctx, task.ID); err != nil {
			return err
		}

		failAt, shouldFail := failPoints[task.Label]
		taskStatus := StatusSucceeded

		for i, name := range diagnosticSteps {
			handle, err := r.StartStep(ctx, task.ID, name)
			if err != nil {
				return err
			}

			time.Sleep(diagnosticStepDelay)
			elapsedMS := strconv.FormatInt(time.Since(handle.started).Milliseconds(), 10)

			if shouldFail && i == failAt {
				stepErr := fmt.Errorf("diagnostic: synthetic failure at step %q for task %q", name, task.Label)
				detail := map[string]string{"elapsed_ms": elapsedMS}
				if err := r.FinishStep(handle, StatusFailed, stepErr.Error(), detail, stepErr); err != nil {
					return err
				}
				taskStatus = StatusFailed
				// The Task is already failed; its remaining steps would
				// simulate work that can no longer matter.
				break
			}

			if err := r.FinishStep(handle, StatusSucceeded, "", map[string]string{"elapsed_ms": elapsedMS}, nil); err != nil {
				return err
			}
		}

		if err := r.FinishTask(ctx, task.ID, taskStatus); err != nil {
			return err
		}
	}

	return nil
}
