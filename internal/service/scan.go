package service

import (
	"context"
	"fmt"
	"purser/internal/ports"
	"strconv"
)

// scanJobKind is the pkg/jobqueue.Executor kind registered for scan Jobs
// — see internal/adapters/pipeline.ScanExecutor, the async counterpart to
// this service. See docs/adr/0024-pipeline-core.md.
const scanJobKind = "scan"

// ScanService triggers the common scan pipeline's walk-hash-queue Job.
// Depends only on ports.JobPublisher and ports.FileWalker — never
// pkg/jobqueue directly, per docs/adr/0023-job-queue.md's "the pipeline
// never imports pkg/jobqueue directly" rule. enableMD5/enableSHA512 are
// plain bools, not a config.Pipeline value, so this package stays free of
// any internal/config dependency, consistent with every other service.
type ScanService struct {
	pub          ports.JobPublisher
	walker       ports.FileWalker
	enableMD5    bool
	enableSHA512 bool
}

// NewScanService constructs a ScanService backed by pub and walker.
func NewScanService(pub ports.JobPublisher, walker ports.FileWalker, enableMD5, enableSHA512 bool) *ScanService {
	return &ScanService{pub: pub, walker: walker, enableMD5: enableMD5, enableSHA512: enableSHA512}
}

// Trigger walks root synchronously (cheap — enumerating paths, not
// hashing) to build one Task label per discovered file, then starts a
// "scan" Job and returns its ID immediately; per-file hashing and
// queueing runs asynchronously in the registered ScanExecutor, the same
// shape as JobService.Trigger.
func (s *ScanService) Trigger(ctx context.Context, root string) (string, error) {
	files, err := s.walker.Walk(ctx, root)
	if err != nil {
		return "", fmt.Errorf("service: walking %s: %w", root, err)
	}

	taskLabels := make([]string, 0, len(files))
	for _, f := range files {
		taskLabels = append(taskLabels, f.Path)
	}

	params := map[string]string{
		"enable_md5":    strconv.FormatBool(s.enableMD5),
		"enable_sha512": strconv.FormatBool(s.enableSHA512),
	}

	return s.pub.Trigger(ctx, scanJobKind, taskLabels, params)
}
