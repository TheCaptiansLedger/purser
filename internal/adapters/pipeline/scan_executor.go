// Package pipeline holds pkgjobqueue.Executor implementations for the
// common scan pipeline — adapter-layer glue between the generic
// pkg/jobqueue engine and pipeline-specific domain/port types. This is a
// deliberate exception to "pipeline/service code never imports
// pkg/jobqueue directly" (docs/adr/0023-job-queue.md): an Executor is
// registered directly on the concrete engine (the same category
// internal/adapters/jobqueue.Adapter and pkg/jobqueue/diagnostic.go
// occupy), not something internal/service ever calls through a port.
// See docs/adr/0024-pipeline-core.md.
package pipeline

import (
	"context"
	"os"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/filehash"
	pkgjobqueue "purser/pkg/jobqueue"
	"strconv"
	"time"
)

const (
	stepHash  = "hash"
	stepQueue = "queue"
)

// ScanExecutor implements pkgjobqueue.Executor for "scan" Jobs. Each
// Task's Label is the discovered file's full path (set by
// internal/service.ScanService.Trigger). Per Task: compute hashes (Step
// "hash"), then create an UnmatchedFile (Step "queue"). It imports
// pkg/jobqueue directly — unlike internal/service/scan.go — because it's
// the adapter glue registered on the concrete *pkgjobqueue.Engine; it also
// needs ports.UnmatchedFileRepository/domain.UnmatchedFile/pkg/filehash,
// a combination pkg/jobqueue itself can never import (its "zero Purser
// knowledge" rule), which is why this type can't live in pkg/jobqueue.
type ScanExecutor struct {
	repo ports.UnmatchedFileRepository
}

var _ pkgjobqueue.Executor = (*ScanExecutor)(nil)

// NewScanExecutor constructs a ScanExecutor backed by repo.
func NewScanExecutor(repo ports.UnmatchedFileRepository) *ScanExecutor {
	return &ScanExecutor{repo: repo}
}

// Execute implements pkgjobqueue.Executor.
func (e *ScanExecutor) Execute(ctx context.Context, r *pkgjobqueue.Runner) error {
	job, err := r.Job(ctx)
	if err != nil {
		return err
	}

	enableMD5, _ := strconv.ParseBool(job.Params["enable_md5"])
	enableSHA512, _ := strconv.ParseBool(job.Params["enable_sha512"])

	for _, task := range job.Tasks {
		if err := r.StartTask(ctx, task.ID); err != nil {
			return err
		}
		if err := e.runTask(ctx, r, task.ID, task.Label, enableMD5, enableSHA512); err != nil {
			return err
		}
	}

	return nil
}

// runTask hashes and queues the file at path (task.Label), driving both
// Steps and the terminal Task status for taskID.
func (e *ScanExecutor) runTask(ctx context.Context, r *pkgjobqueue.Runner, taskID, path string, enableMD5, enableSHA512 bool) error {
	oshash, sha1sum, md5sum, sha512sum, hashDetail, hashErr := computeHashes(path, enableMD5, enableSHA512)

	hashHandle, err := r.StartStep(ctx, taskID, stepHash)
	if err != nil {
		return err
	}
	if hashErr != nil {
		if err := r.FinishStep(hashHandle, pkgjobqueue.StatusFailed, hashErr.Error(), hashDetail, hashErr); err != nil {
			return err
		}
		return r.FinishTask(ctx, taskID, pkgjobqueue.StatusFailed)
	}
	if err := r.FinishStep(hashHandle, pkgjobqueue.StatusSucceeded, "", hashDetail, nil); err != nil {
		return err
	}

	queueHandle, err := r.StartStep(ctx, taskID, stepQueue)
	if err != nil {
		return err
	}

	size := int64(0)
	if info, statErr := os.Stat(path); statErr == nil {
		size = info.Size()
	}
	uf := &domain.UnmatchedFile{
		ID:           domain.NewID(),
		Path:         path,
		Size:         size,
		OSHash:       oshash,
		SHA1:         sha1sum,
		MD5:          md5sum,
		SHA512:       sha512sum,
		DiscoveredAt: time.Now(),
		Status:       domain.UnmatchedFileStatusPending,
	}
	if err := uf.Validate(); err != nil {
		if fErr := r.FinishStep(queueHandle, pkgjobqueue.StatusFailed, err.Error(), nil, err); fErr != nil {
			return fErr
		}
		return r.FinishTask(ctx, taskID, pkgjobqueue.StatusFailed)
	}
	if err := e.repo.Create(ctx, uf); err != nil {
		if fErr := r.FinishStep(queueHandle, pkgjobqueue.StatusFailed, err.Error(), nil, err); fErr != nil {
			return fErr
		}
		return r.FinishTask(ctx, taskID, pkgjobqueue.StatusFailed)
	}

	if err := r.FinishStep(queueHandle, pkgjobqueue.StatusSucceeded, "", map[string]string{"unmatched_file.id": uf.ID}, nil); err != nil {
		return err
	}
	return r.FinishTask(ctx, taskID, pkgjobqueue.StatusSucceeded)
}

// computeHashes runs filehash.OSHash/SHA1 unconditionally and
// filehash.MD5/SHA512 only when enabled, returning a Detail map suitable
// for the "hash" Step and the first error encountered, if any. OSHash
// returning filehash.ErrFileTooSmall is treated as this task's failure —
// a walking skeleton has no fallback identity for files under 64 KiB.
func computeHashes(path string, enableMD5, enableSHA512 bool) (oshash, sha1sum, md5sum, sha512sum string, detail map[string]string, err error) {
	detail = map[string]string{}

	oshash, err = filehash.OSHash(path)
	if err != nil {
		return "", "", "", "", detail, err
	}
	detail["oshash"] = oshash

	sha1sum, err = filehash.SHA1(path)
	if err != nil {
		return "", "", "", "", detail, err
	}
	detail["sha1"] = sha1sum

	if enableMD5 {
		md5sum, err = filehash.MD5(path)
		if err != nil {
			return "", "", "", "", detail, err
		}
		detail["md5"] = md5sum
	}

	if enableSHA512 {
		sha512sum, err = filehash.SHA512(path)
		if err != nil {
			return "", "", "", "", detail, err
		}
		detail["sha512"] = sha512sum
	}

	return oshash, sha1sum, md5sum, sha512sum, detail, nil
}
