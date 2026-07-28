package service

import (
	"context"
	"fmt"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"strconv"
	"strings"
)

// scanJobKind is the pkg/jobqueue.Executor kind registered for scan Jobs
// — see internal/adapters/pipeline.ScanExecutor, the async counterpart to
// this service. See docs/adr/0024-pipeline-core.md.
const scanJobKind = "scan"

// RootContentType pairs a configured scan root with its domain.ContentType
// — ScanService's own plain copy of config.ScanRoot's shape, so this
// package stays free of any internal/config dependency (the composition
// root converts config.Pipeline.ScanRoots into a []RootContentType when
// constructing a ScanService), consistent with every other service.
type RootContentType struct {
	Path        string
	ContentType domain.ContentType
}

// ScanService triggers the common scan pipeline's walk-hash-queue Job.
// Depends only on ports.JobPublisher, ports.FileWalker, and
// ports.SidecarClassifierResolver — never pkg/jobqueue directly, per
// docs/adr/0023-job-queue.md's "the pipeline never imports pkg/jobqueue
// directly" rule. enableMD5/enableSHA512 are plain bools, not a
// config.Pipeline value, so this package stays free of any internal/config
// dependency, consistent with every other service.
type ScanService struct {
	pub          ports.JobPublisher
	walker       ports.FileWalker
	enableMD5    bool
	enableSHA512 bool
	roots        []RootContentType
	classifier   ports.SidecarClassifierResolver
}

// NewScanService constructs a ScanService backed by pub, walker, and
// classifier. roots is consulted by Trigger to resolve a scanned root's
// domain.ContentType via longest-prefix match; a root with no match
// resolves to an empty ContentType, which GroupingRegistry treats as "no
// Grouping registered" and falls back to IdentityGrouping (and
// classifier, symmetrically, falls back to NoopClassifier).
func NewScanService(pub ports.JobPublisher, walker ports.FileWalker, enableMD5, enableSHA512 bool, roots []RootContentType, classifier ports.SidecarClassifierResolver) *ScanService {
	return &ScanService{pub: pub, walker: walker, enableMD5: enableMD5, enableSHA512: enableSHA512, roots: roots, classifier: classifier}
}

// Trigger walks root synchronously (cheap — enumerating paths, not
// hashing), classifies each discovered file (per
// docs/adr/0024-pipeline-core.md's "sidecar/companion assets are classified
// before identification" decision — a file classified as anything other
// than ports.SidecarKindNone never becomes a Task at all) to build one Task
// label per surviving file, then starts a "scan" Job and returns its ID
// immediately; per-file hashing and queueing runs asynchronously in the
// registered ScanExecutor, the same shape as JobService.Trigger.
func (s *ScanService) Trigger(ctx context.Context, root string) (string, error) {
	files, err := s.walker.Walk(ctx, root)
	if err != nil {
		return "", fmt.Errorf("service: walking %s: %w", root, err)
	}

	contentType := s.resolveContentType(root)

	taskLabels := make([]string, 0, len(files))
	for _, f := range files {
		kind, err := s.classifier.Classify(ctx, contentType, f.Path)
		if err != nil {
			return "", fmt.Errorf("service: classifying %s: %w", f.Path, err)
		}
		if kind != ports.SidecarKindNone {
			continue
		}
		taskLabels = append(taskLabels, f.Path)
	}

	params := map[string]string{
		"enable_md5":    strconv.FormatBool(s.enableMD5),
		"enable_sha512": strconv.FormatBool(s.enableSHA512),
		"content_type":  string(contentType),
		"scan_root":     root,
	}

	return s.pub.Trigger(ctx, scanJobKind, taskLabels, params)
}

// resolveContentType finds the configured RootContentType whose Path is
// the longest matching prefix of root — either root's own configured
// root, or (per docs/adr/0024-pipeline-core.md's watcher-triggered
// discovery) a subfolder of one. Returns an empty domain.ContentType if
// nothing configured matches.
//
// Both root and each configured Path are resolved to absolute form before
// comparing (absOrClean), not just filepath.Clean'd — pkg/fswatch always
// hands Trigger an absolute path for a watcher-triggered scan regardless of
// how config.Pipeline.ScanRoots was written, so a relative configured Path
// would otherwise never match and silently resolve to an empty
// ContentType: GroupingRegistry/SidecarClassifierRegistry then fall back to
// their no-op defaults with no error surfaced anywhere. Comparing on
// absolute form closes that gap for any caller (watcher, an on-demand
// TriggerScan RPC, a config file), not just the ones already careful to use
// absolute paths.
func (s *ScanService) resolveContentType(root string) domain.ContentType {
	cleanedRoot := absOrClean(root)

	var best RootContentType
	bestLen := -1
	for _, r := range s.roots {
		cleanedConfigured := absOrClean(r.Path)
		if !isUnderRoot(cleanedRoot, cleanedConfigured) {
			continue
		}
		if len(cleanedConfigured) > bestLen {
			bestLen = len(cleanedConfigured)
			best = r
		}
	}

	return best.ContentType
}

// absOrClean returns p resolved to an absolute path so relative and
// already-absolute forms compare consistently regardless of which one a
// caller or config file used, falling back to filepath.Clean(p) only if the
// working directory can't be resolved (a locked-down environment with no
// resolvable cwd) rather than failing content-type resolution outright.
func absOrClean(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

// isUnderRoot reports whether path is root itself or a descendant of it,
// matching on path segments rather than a raw string prefix so
// "/media/inc" never matches "/media/incoming".
func isUnderRoot(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
