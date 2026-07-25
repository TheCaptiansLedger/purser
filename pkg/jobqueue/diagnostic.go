package jobqueue

import (
	"context"
	"strconv"
	"time"
)

// diagnosticStepDelay is the brief, real delay each synthetic step sleeps
// for, so Detail's elapsed_ms reflects genuine elapsed time rather than a
// hardcoded value.
const diagnosticStepDelay = 50 * time.Millisecond

// diagnosticSteps are the synthetic steps every Task of a "diagnostic" Job
// runs through, in order.
var diagnosticSteps = []string{"initialize", "process", "finalize"}

// diagnosticExecutor is the built-in Executor registered under the
// "diagnostic" kind — a permanent smoke-testing capability of the engine
// (not scaffolding), exercising the real Runner API end-to-end against
// every Task/Step it's given. It always succeeds; failure/partial-status
// handling is explicitly out of scope for this pass. See
// docs/adr/0023-job-queue.md.
type diagnosticExecutor struct{}

func newDiagnosticExecutor() *diagnosticExecutor {
	return &diagnosticExecutor{}
}

// Execute implements Executor.
func (d *diagnosticExecutor) Execute(ctx context.Context, r *Runner) error {
	job, err := r.Job(ctx)
	if err != nil {
		return err
	}

	for _, task := range job.Tasks {
		if err := r.StartTask(ctx, task.ID); err != nil {
			return err
		}

		for _, name := range diagnosticSteps {
			handle, err := r.StartStep(ctx, task.ID, name)
			if err != nil {
				return err
			}

			time.Sleep(diagnosticStepDelay)

			elapsedMS := strconv.FormatInt(time.Since(handle.started).Milliseconds(), 10)
			if err := r.FinishStep(handle, StatusSucceeded, "", map[string]string{"elapsed_ms": elapsedMS}); err != nil {
				return err
			}
		}

		if err := r.FinishTask(ctx, task.ID, StatusSucceeded); err != nil {
			return err
		}
	}

	return nil
}
