// This file is the ports.Identifier implementation for
// domain.ContentTypeAdult — see docs/adr/0024-pipeline-core.md, AD5
// (issue #559), and internal/adapters/pipeline/music/identifier.go (M7),
// the shape this mirrors. Sets Tier/Signals/Metadata only, never Score
// (AD6's job, per ports.Identifier's docstring).
package afterdark

import (
	"context"
	"errors"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Metadata["source"] values every candidate carries, per AD5's verification
// checklist — StashDB and ThePornDB never share an ID space
// (docs/adr/0027-provider-independence.md), so this is how a downstream
// caller (AD6/AD7) tells which provider a candidate's ExternalRef belongs
// to.
const (
	sourceStashDB = "stashdb"
	sourceTPDB    = "tpdb"
)

// Identifier implements ports.Identifier for domain.ContentTypeAdult: the
// AD5 candidate-generation cascade over AD1 (StashDB) + AD2 (ThePornDB) —
// a direct-ID short-circuit, then three independently-collected tiers
// (fingerprint, JAV-code, fuzzy). Unlike Music's Identifier, candidates are
// never deduplicated across the two providers — per ADR-0027, StashDB and
// ThePornDB have no shared ID space, so a StashDB-sourced and a
// ThePornDB-sourced candidate for the same real scene are always kept as
// two separate domain.MatchCandidates; only within one provider's own
// results for one tier are duplicates collapsed (see stashdbFingerprintTier/
// tpdbFingerprintTier).
//
// A real (non-ports.ErrNotFound) error from either provider on any tier is
// logged and that provider is simply skipped for that tier — it never
// aborts the whole Identify call. This is a deliberate difference from
// Music's Identifier (where a MusicBrainz error is fatal): Music has one
// required provider, AfterDark has two independent ones by design, and a
// StashDB outage must not block ThePornDB-only identification, or vice
// versa. Identify therefore never returns a non-nil error.
type Identifier struct {
	stashDB        ports.StashDBClient
	tpdb           ports.ThePornDBClient
	filenameParser ports.FilenameParser

	logger *slog.Logger
	tracer trace.Tracer
}

var _ ports.Identifier = (*Identifier)(nil)

// NewIdentifier constructs an Identifier backed by stashDB, tpdb, and
// filenameParser. Reuses the same Option/WithLogger/WithTracerProvider
// declared in fingerprinter.go — every type in this package shares the
// same {logger, tracerProvider} shape, so a second declaration would be
// pure duplication (same reasoning Music's NewIdentifier doc comment
// gives).
func NewIdentifier(stashDB ports.StashDBClient, tpdb ports.ThePornDBClient, filenameParser ports.FilenameParser, opts ...Option) *Identifier {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Identifier{
		stashDB:        stashDB,
		tpdb:           tpdb,
		filenameParser: filenameParser,
		logger:         o.logger.With("component", "adapters.pipeline.afterdark.identifier"),
		tracer:         o.tracerProvider.Tracer(instrumentationName),
	}
}

// ContentTypes implements ports.Identifier.
func (id *Identifier) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

// Identify implements ports.Identifier. AfterDark reuses IdentityGrouping
// as-is (a group is always exactly one file — see fingerprinter.go), so
// paths[0]/groupPath refer to that same one file; paths is consulted first
// purely because it's the more specific of the two per ports.Identifier's
// contract, falling back to groupPath only if paths is empty.
func (id *Identifier) Identify(ctx context.Context, fp domain.Fingerprint, paths []string, groupPath, scanRoot string) ([]domain.MatchCandidate, error) {
	ctx, span := id.tracer.Start(ctx, "afterdark.identifier.identify", trace.WithAttributes(
		attribute.String("pipeline.group_path", groupPath),
	))
	defer span.End()

	path := groupPath
	if len(paths) > 0 {
		path = paths[0]
	}

	if candidates := id.directID(ctx, path); len(candidates) > 0 {
		span.SetAttributes(attribute.Bool("pipeline.direct_id", true))
		id.logger.DebugContext(ctx, "direct-id short-circuit resolved", "group_path", groupPath, "candidate_count", len(candidates))
		return candidates, nil
	}

	fingerprintCandidates := id.fingerprintTier(ctx, fp)
	javCandidates := id.javCodeTier(ctx, path)
	fuzzyCandidates := id.fuzzyTier(ctx, groupPath, scanRoot)

	candidates := make([]domain.MatchCandidate, 0, len(fingerprintCandidates)+len(javCandidates)+len(fuzzyCandidates))
	candidates = append(candidates, fingerprintCandidates...)
	candidates = append(candidates, javCandidates...)
	candidates = append(candidates, fuzzyCandidates...)

	id.logger.DebugContext(ctx, "identified candidates", "group_path", groupPath, "candidate_count", len(candidates))
	return candidates, nil
}

// directID implements the direct-ID short-circuit: extracts a UUID-shaped
// substring from path (see extractUUID) and looks it up directly on both
// providers. Both are tried regardless of whether one already hit — a
// filename-embedded UUID could, in principle, resolve on both (they don't
// share an ID space, so a coincidental hit on both is not a conflict to
// resolve, just two candidates), though in practice a given UUID belongs to
// at most one provider's database. No UUID found, or neither provider
// recognizes it, returns nil/empty — the caller falls through to the
// fingerprint/JAV-code/fuzzy tiers, never treated as an error.
func (id *Identifier) directID(ctx context.Context, path string) []domain.MatchCandidate {
	uuid, ok := extractUUID(path)
	if !ok {
		return nil
	}

	var candidates []domain.MatchCandidate
	if scene, err := id.stashDB.LookupScene(ctx, uuid); err == nil {
		candidates = append(candidates, directIDCandidate(sourceStashDB, scene.ID, scene.Title))
	} else if !errors.Is(err, ports.ErrNotFound) {
		id.logger.WarnContext(ctx, "stashdb direct-id lookup failed", "uuid", uuid, "error", err)
	}

	if scene, err := id.tpdb.LookupScene(ctx, uuid); err == nil {
		candidates = append(candidates, directIDCandidate(sourceTPDB, scene.ID, scene.Title))
	} else if !errors.Is(err, ports.ErrNotFound) {
		id.logger.WarnContext(ctx, "theporndb direct-id lookup failed", "uuid", uuid, "error", err)
	}

	return candidates
}

func directIDCandidate(source, externalRef, title string) domain.MatchCandidate {
	return domain.MatchCandidate{
		ExternalRef: externalRef,
		Title:       title,
		Tier:        domain.MatchTierDirectID,
		Signals:     map[string]float64{},
		Metadata:    map[string]any{"source": source},
	}
}

// fingerprintTier implements the OSHash+PHash tier: AD3b's computed hashes
// (fp.Metadata["os_hash"]/["phash"] — os_hash may be absent for a too-small
// file, phash is always present on a successfully fingerprinted file) are
// looked up against both providers. Per docs/technical/afterdark-data_model.md
// §5.2, this is AfterDark's primary identification tier (reversed from
// Music, where AcoustID is a fallback) — both providers converge on the
// same hash for the same content.
func (id *Identifier) fingerprintTier(ctx context.Context, fp domain.Fingerprint) []domain.MatchCandidate {
	osHash, _ := fp.Metadata["os_hash"].(string)
	pHash, _ := fp.Metadata["phash"].(string)
	if osHash == "" && pHash == "" {
		return nil
	}

	stashCandidates := id.stashdbFingerprintTier(ctx, osHash, pHash)
	tpdbCandidates := id.tpdbFingerprintTier(ctx, osHash, pHash)

	candidates := make([]domain.MatchCandidate, 0, len(stashCandidates)+len(tpdbCandidates))
	candidates = append(candidates, stashCandidates...)
	candidates = append(candidates, tpdbCandidates...)
	return candidates
}

// stashdbFingerprintTier calls StashDB's batch findScenesBySceneFingerprints
// with every non-empty hash at once. Duplicate scene IDs are collapsed
// (StashDB's own resolver is expected not to return one scene twice for a
// multi-fingerprint query, but this guards it defensively) — StashDB's
// returned order is otherwise preserved, never re-sorted.
func (id *Identifier) stashdbFingerprintTier(ctx context.Context, osHash, pHash string) []domain.MatchCandidate {
	var fingerprints []ports.SceneFingerprint
	if osHash != "" {
		fingerprints = append(fingerprints, ports.SceneFingerprint{Hash: osHash, Algorithm: ports.FingerprintAlgorithmOSHash})
	}
	if pHash != "" {
		fingerprints = append(fingerprints, ports.SceneFingerprint{Hash: pHash, Algorithm: ports.FingerprintAlgorithmPHash})
	}

	scenes, err := id.stashDB.FindScenesByFingerprints(ctx, fingerprints)
	if err != nil {
		id.logger.WarnContext(ctx, "stashdb fingerprint lookup failed", "error", err)
		return nil
	}

	seen := map[string]struct{}{}
	candidates := make([]domain.MatchCandidate, 0, len(scenes))
	for _, scene := range scenes {
		if _, dup := seen[scene.ID]; dup {
			continue
		}
		seen[scene.ID] = struct{}{}
		candidates = append(candidates, fingerprintCandidate(sourceStashDB, scene.ID, scene.Title, stashdbFingerprintSubmissions(scene, osHash, pHash)))
	}
	return candidates
}

// stashdbFingerprintSubmissions sums the submissions count of every
// fingerprint entry on scene that matches osHash or pHash — StashDB's
// crowd-sourced confidence signal, surfaced as Signals["fingerprint_
// submissions"] for AD6's corroboration-bonus formula.
func stashdbFingerprintSubmissions(scene ports.Scene, osHash, pHash string) int {
	total := 0
	for _, f := range scene.Fingerprints {
		if (osHash != "" && f.Hash == osHash) || (pHash != "" && f.Hash == pHash) {
			total += f.Submissions
		}
	}
	return total
}

// tpdbFingerprintTier calls ThePornDB's singular GET /scenes/hash/{hash}
// once per available hash (no batch endpoint exists, unlike StashDB) and
// collapses a scene hit by both hashes into one candidate. Unlike
// stashdbFingerprintTier, ThePornDB's own order isn't meaningful here
// (there is none — each hash is its own independent request), so the
// deduplicated result is sorted by scene ID purely for deterministic
// output, not as a ranking.
func (id *Identifier) tpdbFingerprintTier(ctx context.Context, osHash, pHash string) []domain.MatchCandidate {
	seen := map[string]domain.MatchCandidate{}
	for _, hash := range []string{osHash, pHash} {
		if hash == "" {
			continue
		}
		scene, err := id.tpdb.LookupSceneByHash(ctx, hash)
		if err != nil {
			if !errors.Is(err, ports.ErrNotFound) {
				id.logger.WarnContext(ctx, "theporndb fingerprint lookup failed", "hash", hash, "error", err)
			}
			continue
		}
		seen[scene.ID] = fingerprintCandidate(sourceTPDB, scene.ID, scene.Title, tpdbHashSubmissions(*scene, osHash, pHash))
	}
	if len(seen) == 0 {
		return nil
	}

	ids := make([]string, 0, len(seen))
	for sceneID := range seen {
		ids = append(ids, sceneID)
	}
	sort.Strings(ids)

	candidates := make([]domain.MatchCandidate, 0, len(ids))
	for _, sceneID := range ids {
		candidates = append(candidates, seen[sceneID])
	}
	return candidates
}

// tpdbHashSubmissions mirrors stashdbFingerprintSubmissions for ThePornDB's
// Hashes[] shape.
func tpdbHashSubmissions(scene ports.TPDBScene, osHash, pHash string) int {
	total := 0
	for _, h := range scene.Hashes {
		if (osHash != "" && h.Hash == osHash) || (pHash != "" && h.Hash == pHash) {
			total += h.Submissions
		}
	}
	return total
}

// fingerprintCandidate builds one fingerprint-tier candidate.
// Tier=MatchTierUniqueID: a matching hash is a deterministic, near-1:1
// identifying lookup (both providers converge on the same hash for the
// same content, per docs/technical/afterdark-data_model.md §2), the same
// shape as Music's barcode/ISRC unique_id tier — not MatchTierAcoustic,
// which this package reserves for the JAV-code tier's tolerant external
// resolver call (see javCodeTier).
func fingerprintCandidate(source, externalRef, title string, submissions int) domain.MatchCandidate {
	signals := map[string]float64{"fingerprint_match": 1.0}
	if submissions > 0 {
		signals["fingerprint_submissions"] = float64(submissions)
	}
	return domain.MatchCandidate{
		ExternalRef: externalRef,
		Title:       title,
		Tier:        domain.MatchTierUniqueID,
		Signals:     signals,
		Metadata:    map[string]any{"source": source},
	}
}

// javCodeTier implements the JAV-code tier: AD4's ExtractJAVCode feeds
// ThePornDB's GET /jav?parse= only — StashDB has no equivalent endpoint.
// ports.ThePornDBClient.ResolveJAVCode documents that endpoint as returning
// a ranked, tolerant list (not a single deterministic match, and often
// including scenes that don't share the exact code) — mechanically the
// same shape as Music's AcoustID tier (send a computed value to an external
// resolver, get back a ranked candidate list), so Tier=MatchTierAcoustic,
// not MatchTierUniqueID. Every returned scene becomes its own candidate, in
// ThePornDB's own order — never re-sorted or filtered, per
// docs/adr/0027-provider-independence.md's read-only-passthrough principle.
func (id *Identifier) javCodeTier(ctx context.Context, path string) []domain.MatchCandidate {
	code, ok := ExtractJAVCode(path)
	if !ok {
		return nil
	}

	scenes, err := id.tpdb.ResolveJAVCode(ctx, code)
	if err != nil {
		if !errors.Is(err, ports.ErrNotFound) {
			id.logger.WarnContext(ctx, "theporndb jav-code lookup failed", "code", code, "error", err)
		}
		return nil
	}

	candidates := make([]domain.MatchCandidate, 0, len(scenes))
	for _, scene := range scenes {
		candidates = append(candidates, domain.MatchCandidate{
			ExternalRef: scene.ID,
			Title:       scene.Title,
			Tier:        domain.MatchTierAcoustic,
			Signals:     map[string]float64{"jav_code_match": 1.0},
			Metadata:    map[string]any{"source": sourceTPDB, "jav_code": code},
		})
	}
	return candidates
}

// fuzzyTier implements the studio/title free-text tier: AD4's
// FilenameParser guess feeds both providers' scene title search. studio is
// never itself a search query (neither StashDB nor ThePornDB expose
// free-text studio search — see ports.StashDBClient.LookupStudio's doc
// comment), only a scoring signal against each result's own studio/site
// name.
func (id *Identifier) fuzzyTier(ctx context.Context, groupPath, scanRoot string) []domain.MatchCandidate {
	studio, title, ok := id.filenameParser.Parse(ctx, groupPath, scanRoot)
	if !ok || title == "" {
		return nil
	}

	stashCandidates := id.stashdbFuzzyTier(ctx, studio, title)
	tpdbCandidates := id.tpdbFuzzyTier(ctx, studio, title)

	candidates := make([]domain.MatchCandidate, 0, len(stashCandidates)+len(tpdbCandidates))
	candidates = append(candidates, stashCandidates...)
	candidates = append(candidates, tpdbCandidates...)
	return candidates
}

func (id *Identifier) stashdbFuzzyTier(ctx context.Context, studio, title string) []domain.MatchCandidate {
	scenes, err := id.stashDB.SearchScenes(ctx, title)
	if err != nil {
		id.logger.WarnContext(ctx, "stashdb fuzzy search failed", "title", title, "error", err)
		return nil
	}

	candidates := make([]domain.MatchCandidate, 0, len(scenes))
	for _, scene := range scenes {
		candidates = append(candidates, fuzzyCandidate(sourceStashDB, scene.ID, scene.Title, stashdbSceneStudioName(scene), title, studio))
	}
	return candidates
}

func stashdbSceneStudioName(scene ports.Scene) string {
	if scene.Studio != nil {
		return scene.Studio.Name
	}
	return ""
}

func (id *Identifier) tpdbFuzzyTier(ctx context.Context, studio, title string) []domain.MatchCandidate {
	scenes, err := id.tpdb.SearchScenes(ctx, title)
	if err != nil {
		id.logger.WarnContext(ctx, "theporndb fuzzy search failed", "title", title, "error", err)
		return nil
	}

	candidates := make([]domain.MatchCandidate, 0, len(scenes))
	for _, scene := range scenes {
		candidates = append(candidates, fuzzyCandidate(sourceTPDB, scene.ID, scene.Title, tpdbSceneStudioName(scene), title, studio))
	}
	return candidates
}

func tpdbSceneStudioName(scene ports.TPDBScene) string {
	if scene.Site != nil {
		return scene.Site.Name
	}
	return ""
}

// fuzzyCandidate builds one fuzzy-tier candidate, scoring candidateTitle/
// candidateStudio (the provider's own result) against queryTitle/
// queryStudio (AD4's filename guess) via nameSimilarity. A signal is only
// present when both sides had a non-empty value to compare — absent, never
// zero, when it couldn't be evaluated, matching the M7/M8 signal-key
// contract Music's identifier already established.
func fuzzyCandidate(source, externalRef, candidateTitle, candidateStudio, queryTitle, queryStudio string) domain.MatchCandidate {
	signals := map[string]float64{}
	if queryTitle != "" && candidateTitle != "" {
		signals["title_similarity"] = nameSimilarity(queryTitle, candidateTitle)
	}
	if queryStudio != "" && candidateStudio != "" {
		signals["studio_similarity"] = nameSimilarity(queryStudio, candidateStudio)
	}
	return domain.MatchCandidate{
		ExternalRef: externalRef,
		Title:       candidateTitle,
		Tier:        domain.MatchTierFuzzy,
		Signals:     signals,
		Metadata:    map[string]any{"source": source},
	}
}

// nameSimilarity is a case-insensitive, whitespace-trimmed similarity score
// in [0, 1] based on normalized Levenshtein distance — same algorithm, and
// same "not asserted correct, a starting point" caveat, as Music's own
// unexported nameSimilarity (internal/adapters/pipeline/music/identifier.go).
// Duplicated rather than shared via pkg/: each pipeline content-type
// package stays self-contained, the same convention AD4's FilenameParser
// already follows by re-implementing Music's M6 shape instead of importing
// it.
func nameSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	maxLen := len([]rune(a))
	if l := len([]rune(b)); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 1
	}
	score := 1 - float64(fuzzy.LevenshteinDistance(a, b))/float64(maxLen)
	if score < 0 {
		return 0
	}
	return score
}
