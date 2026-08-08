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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/filehash"
	pkgjobqueue "purser/pkg/jobqueue"
	"strconv"
	"time"
)

const (
	stepHash        = "hash"
	stepFingerprint = "fingerprint"
	stepCheckKnown  = "check_known"
	stepQueue       = "queue"
	stepDecide      = "decide"

	// outcome values recorded on the "check_known" Step's Detail — see
	// checkKnown and docs/adr/0024-pipeline-core.md's "already known"
	// short-circuit.
	outcomeMatchedMediaFile     = "matched_media_file"
	outcomeMatchedUnmatchedFile = "matched_unmatched_file"
	outcomeNew                  = "new"
)

// decisionResolver is the narrow interface decideAndPersistGroups depends
// on — satisfied structurally by *service.DecisionService without this
// adapter-layer package importing internal/service (ScanExecutor already
// depends on ports for everything else; this keeps that same direction for
// the one piece decide/persist needs that isn't itself a port). See
// docs/technical/pipeline-music-persist.md.
type decisionResolver interface {
	Decide(ctx context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidates []domain.MatchCandidate, files []*domain.UnmatchedFile) (persisted bool, err error)
}

// ScanExecutor implements pkgjobqueue.Executor for "scan" Jobs. Each
// Task's Label is the discovered file's full path (set by
// internal/service.ScanService.Trigger). Per Task: compute hashes (Step
// "hash"), extract per-file identification data (Step "fingerprint", non-
// fatal — see runFingerprintStep), then check whether the file is already
// known by hash (Step "check_known") before either short-circuiting (a
// MediaFile or UnmatchedFile hit just gets its Path updated) or creating a
// new UnmatchedFile (Step "queue"). Once every Task has run, buffered
// per-file Fingerprints are reduced to one consensus Fingerprint per
// GroupKey, written onto every UnmatchedFile row in that group, and fed
// through identify -> score -> decide -> (on success) delete — see
// decideAndPersistGroups and docs/technical/pipeline-music-fingerprinter.md's
// "Group consensus" section / docs/technical/pipeline-music-persist.md's
// "Where this runs" section. It imports pkg/jobqueue directly — unlike
// internal/service/scan.go — because it's the adapter glue registered on
// the concrete *pkgjobqueue.Engine; it also needs
// ports.UnmatchedFileRepository/ports.MediaFileRepository/
// domain.UnmatchedFile/pkg/filehash, a combination pkg/jobqueue itself can
// never import (its "zero Purser knowledge" rule), which is why this type
// can't live in pkg/jobqueue.
type ScanExecutor struct {
	repo            ports.UnmatchedFileRepository
	mediaFileRepo   ports.MediaFileRepository
	grouping        ports.GroupingResolver
	fingerprinter   ports.FileFingerprinterResolver
	identifier      ports.IdentifierResolver
	confidenceScore ports.ConfidenceScoreResolver
	decision        decisionResolver
}

var _ pkgjobqueue.Executor = (*ScanExecutor)(nil)

// NewScanExecutor constructs a ScanExecutor backed by repo, mediaFileRepo,
// grouping, fingerprinter, identifier, confidenceScore, and decision.
func NewScanExecutor(
	repo ports.UnmatchedFileRepository,
	mediaFileRepo ports.MediaFileRepository,
	grouping ports.GroupingResolver,
	fingerprinter ports.FileFingerprinterResolver,
	identifier ports.IdentifierResolver,
	confidenceScore ports.ConfidenceScoreResolver,
	decision decisionResolver,
) *ScanExecutor {
	return &ScanExecutor{
		repo: repo, mediaFileRepo: mediaFileRepo, grouping: grouping, fingerprinter: fingerprinter,
		identifier: identifier, confidenceScore: confidenceScore, decision: decision,
	}
}

// Execute implements pkgjobqueue.Executor. Grouping runs once for the
// whole Job, not once per file — the multi-disc roll-up a content type's
// Grouping implementation may perform needs to see every task's label
// before deciding whether sibling disc-subfolders should share one group,
// per docs/adr/0024-pipeline-core.md.
func (e *ScanExecutor) Execute(ctx context.Context, r *pkgjobqueue.Runner) error {
	job, err := r.Job(ctx)
	if err != nil {
		return err
	}

	enableMD5, _ := strconv.ParseBool(job.Params["enable_md5"])
	enableSHA512, _ := strconv.ParseBool(job.Params["enable_sha512"])
	contentType := domain.ContentType(job.Params["content_type"])
	scanRoot := job.Params["scan_root"]

	paths := make([]string, len(job.Tasks))
	for i, task := range job.Tasks {
		paths[i] = task.Label
	}
	groupResults, err := e.grouping.GroupKeys(ctx, contentType, paths)
	if err != nil {
		return fmt.Errorf("pipeline: grouping paths: %w", err)
	}

	fingerprintsByGroup := make(map[string][]domain.Fingerprint)
	taskIDsByGroup := make(map[string][]string)
	for _, task := range job.Tasks {
		if err := r.StartTask(ctx, task.ID); err != nil {
			return err
		}
		groupResult, ok := groupResults[task.Label]
		if !ok {
			groupResult = ports.GroupingResult{GroupKey: task.Label, DiscNumber: 0}
		}
		taskIDsByGroup[groupResult.GroupKey] = append(taskIDsByGroup[groupResult.GroupKey], task.ID)
		if err := e.runTask(ctx, r, task.ID, task.Label, enableMD5, enableSHA512, contentType, groupResult, fingerprintsByGroup); err != nil {
			return err
		}
	}

	return e.decideAndPersistGroups(ctx, r, contentType, scanRoot, fingerprintsByGroup, taskIDsByGroup)
}

// runTask hashes the file at path (task.Label), extracts its per-file
// Fingerprint (buffered into fingerprintsByGroup for the post-loop
// consensus pass), checks whether it's already known by hash, and either
// short-circuits or queues it as a new UnmatchedFile — driving every Step
// and the terminal Task status for taskID.
func (e *ScanExecutor) runTask(ctx context.Context, r *pkgjobqueue.Runner, taskID, path string, enableMD5, enableSHA512 bool, contentType domain.ContentType, groupResult ports.GroupingResult, fingerprintsByGroup map[string][]domain.Fingerprint) error {
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

	fp, ok, err := e.runFingerprintStep(ctx, r, taskID, path, contentType, groupResult.DiscNumber)
	if err != nil {
		return err
	}
	discNumber, trackNumber := groupResult.DiscNumber, ""
	if ok {
		fingerprintsByGroup[groupResult.GroupKey] = append(fingerprintsByGroup[groupResult.GroupKey], fp)
		discNumber, trackNumber = trackPositionOf(fp, groupResult.DiscNumber)
	}

	proceed, err := e.runCheckKnownStep(ctx, r, taskID, path, oshash, sha1sum, md5sum, sha512sum)
	if err != nil || !proceed {
		return err
	}

	return e.runQueueStep(ctx, r, taskID, path, oshash, sha1sum, md5sum, sha512sum, contentType, groupResult.GroupKey, discNumber, trackNumber)
}

// trackPositionOf reads the per-file disc_number/track_number a
// FileFingerprinter already computed into fp.Metadata (DISCNUMBER-tag
// override applied, per FileFingerprinter.Fingerprint's own doc comment)
// back out for the UnmatchedFile row being queued — fp.Metadata is
// map[string]any, so disc_number decodes as int only within the same
// process that just set it (never round-tripped through storage here, so
// no float64 case is needed, unlike domain.Item.Metadata elsewhere).
// fallbackDisc (the grouping guess) is used when no fingerprinter is
// registered for this content type (NoopFingerprinter leaves
// fp.Metadata empty).
func trackPositionOf(fp domain.Fingerprint, fallbackDisc int) (discNumber int, trackNumber string) {
	discNumber = fallbackDisc
	if d, ok := fp.Metadata["disc_number"].(int); ok {
		discNumber = d
	}
	trackNumber, _ = fp.Metadata["track_number"].(string)
	return discNumber, trackNumber
}

// runFingerprintStep runs the "fingerprint" Step. Unlike hash/check_known/
// queue, a fingerprinting failure (a corrupt file, a missing ffprobe
// binary) is recorded on the Step but never fails the Task — one
// unreadable file still gets queued for review and doesn't block the rest
// of its group's consensus; it simply contributes no identification
// signal, the same "no cost, no crash" treatment an unregistered content
// type already gets from NoopFingerprinter. The returned bool reports
// whether fp is valid and should be buffered for the consensus pass; a
// non-nil error means a Runner/Step-tracking failure that must abort the
// Task like every other step here.
func (e *ScanExecutor) runFingerprintStep(ctx context.Context, r *pkgjobqueue.Runner, taskID, path string, contentType domain.ContentType, discNumberGuess int) (domain.Fingerprint, bool, error) {
	handle, err := r.StartStep(ctx, taskID, stepFingerprint)
	if err != nil {
		return domain.Fingerprint{}, false, err
	}

	fp, fpErr := e.fingerprinter.Fingerprint(ctx, contentType, path, discNumberGuess)
	if fpErr != nil {
		if err := r.FinishStep(handle, pkgjobqueue.StatusFailed, fpErr.Error(), nil, fpErr); err != nil {
			return domain.Fingerprint{}, false, err
		}
		return domain.Fingerprint{}, false, nil
	}
	if err := r.FinishStep(handle, pkgjobqueue.StatusSucceeded, "", map[string]string{"tag_count": strconv.Itoa(len(fp.Tags))}, nil); err != nil {
		return domain.Fingerprint{}, false, err
	}
	return fp, true, nil
}

// decideAndPersistGroups reduces every GroupKey's buffered per-file
// Fingerprints into one consensus Fingerprint (FileFingerprinterResolver.Consensus),
// writes it onto every UnmatchedFile row currently in that group — not just
// the rows this Job's tasks created, since a group can also contain rows
// from an earlier run (per docs/technical/pipeline-music-fingerprinter.md's
// "Group consensus" section) — then runs identify -> score -> decide for
// the group and, on a successful auto-import, deletes its rows out of the
// review queue (decideAndPersistGroup). A content type with no real
// fingerprinter registered (NoopFingerprinter) produces an empty consensus
// Fingerprint for every group, which is deliberately skipped entirely — no
// consensus write and no identify/decide attempt; content types this
// feature doesn't touch yet get no writes at all.
func (e *ScanExecutor) decideAndPersistGroups(ctx context.Context, r *pkgjobqueue.Runner, contentType domain.ContentType, scanRoot string, fingerprintsByGroup map[string][]domain.Fingerprint, taskIDsByGroup map[string][]string) error {
	var groupErrs error
	for groupKey, fingerprints := range fingerprintsByGroup {
		consensus, err := e.fingerprinter.Consensus(ctx, contentType, fingerprints)
		if err != nil {
			return fmt.Errorf("pipeline: computing fingerprint consensus for group %q: %w", groupKey, err)
		}
		if len(consensus.Tags) == 0 && len(consensus.Metadata) == 0 {
			continue
		}

		rows, err := e.repo.ListByGroupKey(ctx, groupKey)
		if err != nil {
			return fmt.Errorf("pipeline: listing group %q for consensus: %w", groupKey, err)
		}
		if len(rows) == 0 {
			continue
		}
		for _, row := range rows {
			row.Fingerprint = &consensus
		}
		if err := e.repo.UpdateBatch(ctx, rows); err != nil {
			return fmt.Errorf("pipeline: persisting fingerprint consensus for group %q: %w", groupKey, err)
		}

		if err := e.decideAndPersistGroup(ctx, r, contentType, scanRoot, groupKey, consensus, rows, taskIDsByGroup[groupKey]); err != nil {
			var groupErr *groupBusinessError
			if errors.As(err, &groupErr) {
				// This group's own identify/score/decide pass failed
				// (already recorded as a failed "decide" Step on every
				// task in the group) — a bad MusicBrainz response, a
				// transient timeout, or similar doesn't get to take
				// every other group in this Job down with it. Keep
				// going, and surface the failure at the Job level once
				// every group has had its own independent attempt.
				groupErrs = errors.Join(groupErrs, err)
				continue
			}
			// Anything else here is a Runner/store failure (StartStep,
			// FinishStep, DeleteBatch) — step-tracking itself can no
			// longer be trusted, so this does abort the whole Job,
			// unlike a groupBusinessError.
			return err
		}
	}
	return groupErrs
}

// groupBusinessError marks a decideAndPersistGroup failure that has
// already been recorded on its group's "decide" Steps (identify, score, or
// Decide itself returning an error) — decideAndPersistGroups uses this
// marker to keep processing the Job's remaining groups instead of
// aborting the whole Job over one group's failure. An unmarked error
// means Step-tracking (the Runner) itself failed, which still aborts
// immediately — see decideAndPersistGroups.
type groupBusinessError struct{ err error }

func (e *groupBusinessError) Error() string { return e.err.Error() }
func (e *groupBusinessError) Unwrap() error { return e.err }

// decideAndPersistGroup runs one group's identify -> score -> decide pass
// and, on a successful auto-import, deletes its rows via DeleteBatch — the
// same "leaves the review queue by deletion" behavior Resolve/
// AcceptCandidate already have, applied to the automatic path. The outcome
// is recorded on every task's "decide" Step in the group — pkg/jobqueue
// has no job-level step concept, so this is deliberately redundant across
// every task, per docs/technical/pipeline-music-persist.md's "Where this
// runs" section (the same convention M7's own group-level work already
// follows).
//
// A group that isn't auto-imported (below threshold, or Decide itself
// failed) has its real scored candidates saved onto its rows via
// UpdateBatch before returning — domain.UnmatchedFile.Candidates has
// existed since M1b, and AcceptCandidateRequest's own doc comment already
// describes "a human picks any ranked candidate" as the primary review
// flow, but nothing ever actually wrote a score into it: GetUnmatchedFile/
// ListUnmatchedFiles/ListGroupUnmatchedFiles returned an empty Candidates
// list for every group that didn't clear the threshold, 100% of the time
// — the real match and its confidence score were computed, then thrown
// away. See docs/technical/pipeline-music-organizer.md's manual
// verification and purser#522.
func (e *ScanExecutor) decideAndPersistGroup(ctx context.Context, r *pkgjobqueue.Runner, contentType domain.ContentType, scanRoot, groupKey string, consensus domain.Fingerprint, rows []*domain.UnmatchedFile, taskIDs []string) error {
	paths := make([]string, len(rows))
	for i, row := range rows {
		paths[i] = row.Path
	}

	candidates, err := e.identifier.Identify(ctx, contentType, consensus, paths, groupKey, scanRoot)
	if err != nil {
		return e.finishDecideSteps(ctx, r, taskIDs, nil, false, fmt.Errorf("pipeline: identifying group %q: %w", groupKey, err))
	}

	scored, err := e.confidenceScore.ConfidenceScore(ctx, contentType, consensus, candidates)
	if err != nil {
		return e.finishDecideSteps(ctx, r, taskIDs, nil, false, fmt.Errorf("pipeline: scoring group %q: %w", groupKey, err))
	}

	persisted, decideErr := e.decision.Decide(ctx, contentType, &consensus, scored, rows)
	if !persisted {
		for _, row := range rows {
			row.Candidates = scored
		}
		if err := e.repo.UpdateBatch(ctx, rows); err != nil {
			return fmt.Errorf("pipeline: persisting candidates for group %q: %w", groupKey, err)
		}
	}
	if decideErr != nil {
		return e.finishDecideSteps(ctx, r, taskIDs, scored, false, fmt.Errorf("pipeline: deciding group %q: %w", groupKey, decideErr))
	}

	if err := e.finishDecideSteps(ctx, r, taskIDs, scored, persisted, nil); err != nil {
		return err
	}
	if !persisted {
		return nil
	}

	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	if err := e.repo.DeleteBatch(ctx, ids); err != nil {
		return fmt.Errorf("pipeline: deleting persisted group %q: %w", groupKey, err)
	}
	return nil
}

// finishDecideSteps records the "decide" Step's outcome (candidate_count,
// persisted, and the full scored candidate list) on every task in the
// group, then returns cause wrapped in groupBusinessError (nil on
// success) — the one place decideAndPersistGroup both reports
// step-tracking and propagates whatever caused it to be called. cause
// comes back wrapped, not raw, so decideAndPersistGroups' caller can tell
// "this group's own business logic failed, already recorded on its
// Steps" apart from a Runner/step-tracking failure (the unwrapped errors
// StartStep/FinishStep themselves can return below) — only the latter
// should abort the whole Job.
//
// Detail["candidates"] is every scored domain.MatchCandidate (ExternalRef,
// Title, Tier, Score, Signals, Metadata), JSON-encoded — per ADR-0023's
// own framing of Step.Detail ("a matched MBID, a computed confidence
// score"), this is exactly what it's for. Real, inspectable Job data via
// JobService.GetJob/ListJobs, replacing what purser#522's manual
// verification was previously pulling out with a temporary slog line:
// why a group didn't clear the auto-import threshold is now answerable
// from the Job itself, not by re-running the pipeline with debug logging
// bolted on.
func (e *ScanExecutor) finishDecideSteps(ctx context.Context, r *pkgjobqueue.Runner, taskIDs []string, candidates []domain.MatchCandidate, persisted bool, cause error) error {
	status := pkgjobqueue.StatusSucceeded
	message := ""
	if cause != nil {
		status = pkgjobqueue.StatusFailed
		message = cause.Error()
	}
	detail := map[string]string{
		"candidate_count": strconv.Itoa(len(candidates)),
		"persisted":       strconv.FormatBool(persisted),
	}
	if len(candidates) > 0 {
		// Best-effort: domain.MatchCandidate is plain scalars/maps, so
		// this realistically never fails, but a bad value in some
		// future Identifier's Metadata shouldn't block persistence
		// over a Detail field. The failure itself stays visible on the
		// Job (not silently dropped) rather than needing a logger
		// dependency this type doesn't otherwise carry.
		if encoded, err := json.Marshal(candidates); err != nil {
			detail["candidates_error"] = err.Error()
		} else {
			detail["candidates"] = string(encoded)
		}
	}

	for _, taskID := range taskIDs {
		handle, err := r.StartStep(ctx, taskID, stepDecide)
		if err != nil {
			return err
		}
		if err := r.FinishStep(handle, status, message, detail, cause); err != nil {
			return err
		}
	}
	if cause == nil {
		return nil
	}
	return &groupBusinessError{cause}
}

// runCheckKnownStep runs the "check_known" Step: a MediaFile or
// UnmatchedFile hit short-circuits (updates that record's Path, finishes
// the Task successfully) and reports proceed=false; a miss records
// outcome=new and reports proceed=true so runTask moves on to "queue". A
// repository error finishes the Step and Task as failed and reports
// proceed=false with a non-nil error.
func (e *ScanExecutor) runCheckKnownStep(ctx context.Context, r *pkgjobqueue.Runner, taskID, path, oshash, sha1sum, md5sum, sha512sum string) (proceed bool, err error) {
	checkHandle, err := r.StartStep(ctx, taskID, stepCheckKnown)
	if err != nil {
		return false, err
	}

	detail, matched, checkErr := e.checkKnown(ctx, path, oshash, sha1sum, md5sum, sha512sum)
	if checkErr != nil {
		if fErr := r.FinishStep(checkHandle, pkgjobqueue.StatusFailed, checkErr.Error(), nil, checkErr); fErr != nil {
			return false, fErr
		}
		return false, r.FinishTask(ctx, taskID, pkgjobqueue.StatusFailed)
	}
	if matched {
		if err := r.FinishStep(checkHandle, pkgjobqueue.StatusSucceeded, "", detail, nil); err != nil {
			return false, err
		}
		return false, r.FinishTask(ctx, taskID, pkgjobqueue.StatusSucceeded)
	}
	if err := r.FinishStep(checkHandle, pkgjobqueue.StatusSucceeded, "", map[string]string{"outcome": outcomeNew}, nil); err != nil {
		return false, err
	}
	return true, nil
}

// runQueueStep runs the "queue" Step for a genuinely new file: creates
// and persists a new domain.UnmatchedFile, driving the Step and the
// terminal Task status. contentType is stamped onto the row so a later,
// job-independent AcceptCandidate call can still tell which Persister to
// dispatch to — see domain.UnmatchedFile.ContentType. discNumber/
// trackNumber are the per-file position trackPositionOf already resolved
// (the fingerprint step's DISCNUMBER-tag override applied, and the
// TRACKNUMBER tag carried straight through — see trackPositionOf's doc
// comment) — required for the Music Persister's tracklist matching
// (docs/technical/pipeline-music-persist.md's step 6) to ever find a real
// scanned file.
func (e *ScanExecutor) runQueueStep(ctx context.Context, r *pkgjobqueue.Runner, taskID, path, oshash, sha1sum, md5sum, sha512sum string, contentType domain.ContentType, groupKey string, discNumber int, trackNumber string) error {
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
		ContentType:  contentType,
		GroupKey:     groupKey,
		DiscNumber:   discNumber,
		TrackNumber:  trackNumber,
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

// checkKnown implements ADR-0024's "already known" short-circuit: a
// MediaFile hit (a file already linked to a real Item that moved on disk)
// or an UnmatchedFile hit (a file still sitting in the review queue that
// moved before anyone resolved it) gets just its Path updated in place,
// and checkKnown returns the Detail map to record on the "check_known"
// Step alongside matched=true. A miss on both returns matched=false,
// telling the caller to proceed with the existing create-new-UnmatchedFile
// flow. MediaFile is checked before UnmatchedFile, per
// docs/adr/0024-pipeline-core.md.
func (e *ScanExecutor) checkKnown(ctx context.Context, path, oshash, sha1sum, md5sum, sha512sum string) (detail map[string]string, matched bool, err error) {
	mf, err := e.mediaFileRepo.GetByHash(ctx, oshash, sha1sum, md5sum, sha512sum)
	switch {
	case err == nil:
		mf.Path = path
		if err := e.mediaFileRepo.Update(ctx, mf); err != nil {
			return nil, false, err
		}
		return map[string]string{"outcome": outcomeMatchedMediaFile, "media_file.id": mf.ID}, true, nil
	case !errors.Is(err, ports.ErrNotFound):
		return nil, false, err
	}

	uf, err := e.repo.GetByHash(ctx, oshash, sha1sum, md5sum, sha512sum)
	switch {
	case err == nil:
		uf.Path = path
		if err := e.repo.Update(ctx, uf); err != nil {
			return nil, false, err
		}
		return map[string]string{"outcome": outcomeMatchedUnmatchedFile, "unmatched_file.id": uf.ID}, true, nil
	case !errors.Is(err, ports.ErrNotFound):
		return nil, false, err
	}

	return nil, false, nil
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
