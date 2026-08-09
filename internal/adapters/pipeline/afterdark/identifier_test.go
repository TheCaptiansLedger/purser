package afterdark_test

import (
	"context"
	"errors"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// fakeStashDB is a hand-rolled ports.StashDBClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule — no HTTP fixture server needed for Identifier logic
// tests. Every unconfigured lookup returns ports.ErrNotFound / an empty
// slice, matching the real adapter's documented "empty is not an error"
// search semantics. The *Calls counters exist purely to verify the
// direct-ID short-circuit actually skips the later tiers, not just that it
// returns the right candidates.
type fakeStashDB struct {
	scenesByID          map[string]ports.Scene
	scenesByFingerprint []ports.Scene
	scenesByTerm        map[string][]ports.Scene

	lookupSceneErr              error
	findScenesByFingerprintsErr error
	searchScenesErr             error

	findScenesByFingerprintsCalls int
	searchScenesCalls             int
}

var _ ports.StashDBClient = (*fakeStashDB)(nil)

func newFakeStashDB() *fakeStashDB {
	return &fakeStashDB{
		scenesByID:   map[string]ports.Scene{},
		scenesByTerm: map[string][]ports.Scene{},
	}
}

func (f *fakeStashDB) LookupPerformer(context.Context, string) (*ports.Performer, error) {
	return nil, ports.ErrNotFound
}

func (f *fakeStashDB) SearchPerformers(context.Context, string) ([]ports.Performer, error) {
	return nil, nil
}

func (f *fakeStashDB) LookupStudio(context.Context, string) (*ports.Studio, error) {
	return nil, ports.ErrNotFound
}

func (f *fakeStashDB) LookupScene(_ context.Context, id string) (*ports.Scene, error) {
	if f.lookupSceneErr != nil {
		return nil, f.lookupSceneErr
	}
	if s, ok := f.scenesByID[id]; ok {
		return &s, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeStashDB) SearchScenes(_ context.Context, term string) ([]ports.Scene, error) {
	f.searchScenesCalls++
	if f.searchScenesErr != nil {
		return nil, f.searchScenesErr
	}
	return f.scenesByTerm[term], nil
}

func (f *fakeStashDB) FindScenesByFingerprints(_ context.Context, _ []ports.SceneFingerprint) ([]ports.Scene, error) {
	f.findScenesByFingerprintsCalls++
	if f.findScenesByFingerprintsErr != nil {
		return nil, f.findScenesByFingerprintsErr
	}
	return f.scenesByFingerprint, nil
}

// fakeThePornDB is a hand-rolled ports.ThePornDBClient double — see
// fakeStashDB's doc comment.
type fakeThePornDB struct {
	scenesByID   map[string]ports.TPDBScene
	scenesByHash map[string]ports.TPDBScene
	scenesByTerm map[string][]ports.TPDBScene
	javByCode    map[string][]ports.TPDBScene

	lookupSceneErr       error
	lookupSceneByHashErr error
	searchScenesErr      error
	resolveJAVCodeErr    error

	lookupSceneByHashCalls int
	resolveJAVCodeCalls    int
	searchScenesCalls      int
}

var _ ports.ThePornDBClient = (*fakeThePornDB)(nil)

func newFakeThePornDB() *fakeThePornDB {
	return &fakeThePornDB{
		scenesByID:   map[string]ports.TPDBScene{},
		scenesByHash: map[string]ports.TPDBScene{},
		scenesByTerm: map[string][]ports.TPDBScene{},
		javByCode:    map[string][]ports.TPDBScene{},
	}
}

func (f *fakeThePornDB) LookupPerformer(context.Context, string) (*ports.TPDBPerformer, error) {
	return nil, ports.ErrNotFound
}

func (f *fakeThePornDB) SearchPerformers(context.Context, string) ([]ports.TPDBPerformer, error) {
	return nil, nil
}

func (f *fakeThePornDB) LookupScene(_ context.Context, id string) (*ports.TPDBScene, error) {
	if f.lookupSceneErr != nil {
		return nil, f.lookupSceneErr
	}
	if s, ok := f.scenesByID[id]; ok {
		return &s, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeThePornDB) SearchScenes(_ context.Context, term string) ([]ports.TPDBScene, error) {
	f.searchScenesCalls++
	if f.searchScenesErr != nil {
		return nil, f.searchScenesErr
	}
	return f.scenesByTerm[term], nil
}

func (f *fakeThePornDB) LookupSceneByHash(_ context.Context, hash string) (*ports.TPDBScene, error) {
	f.lookupSceneByHashCalls++
	if f.lookupSceneByHashErr != nil {
		return nil, f.lookupSceneByHashErr
	}
	if s, ok := f.scenesByHash[hash]; ok {
		return &s, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeThePornDB) ResolveJAVCode(_ context.Context, code string) ([]ports.TPDBScene, error) {
	f.resolveJAVCodeCalls++
	if f.resolveJAVCodeErr != nil {
		return nil, f.resolveJAVCodeErr
	}
	return f.javByCode[code], nil
}

func TestIdentifier_ContentTypes(t *testing.T) {
	id := afterdark.NewIdentifier(newFakeStashDB(), newFakeThePornDB(), afterdark.FilenameParser{})
	got := id.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

func candidateBySource(candidates []domain.MatchCandidate, source string) (domain.MatchCandidate, bool) {
	for _, c := range candidates {
		if s, _ := c.Metadata["source"].(string); s == source {
			return c, true
		}
	}
	return domain.MatchCandidate{}, false
}

func TestIdentifier_Identify_DirectID_SingleProviderShortCircuits(t *testing.T) {
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	stashDB := newFakeStashDB()
	stashDB.scenesByID[uuid] = ports.Scene{ID: uuid, Title: "A Real Scene"}
	tpdb := newFakeThePornDB()

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	path := "/scenes/" + uuid + ".mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("Identify() returned %d candidates, want 1: %+v", len(candidates), candidates)
	}
	c := candidates[0]
	if c.ExternalRef != uuid || c.Tier != domain.MatchTierDirectID {
		t.Errorf("candidate = %+v, want ExternalRef=%s Tier=%s", c, uuid, domain.MatchTierDirectID)
	}
	if source, _ := c.Metadata["source"].(string); source != "stashdb" {
		t.Errorf("Metadata[source] = %v, want stashdb", c.Metadata["source"])
	}

	// The short-circuit must skip every later tier entirely.
	if stashDB.findScenesByFingerprintsCalls != 0 || stashDB.searchScenesCalls != 0 {
		t.Errorf("direct-ID hit still ran later stashdb tiers: fingerprint=%d search=%d", stashDB.findScenesByFingerprintsCalls, stashDB.searchScenesCalls)
	}
	if tpdb.lookupSceneByHashCalls != 0 || tpdb.resolveJAVCodeCalls != 0 || tpdb.searchScenesCalls != 0 {
		t.Errorf("direct-ID hit still ran later tpdb tiers: hash=%d jav=%d search=%d", tpdb.lookupSceneByHashCalls, tpdb.resolveJAVCodeCalls, tpdb.searchScenesCalls)
	}
}

func TestIdentifier_Identify_DirectID_BothProvidersHit(t *testing.T) {
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	stashDB := newFakeStashDB()
	stashDB.scenesByID[uuid] = ports.Scene{ID: uuid, Title: "StashDB Title"}
	tpdb := newFakeThePornDB()
	tpdb.scenesByID[uuid] = ports.TPDBScene{ID: uuid, Title: "ThePornDB Title"}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	path := "/scenes/" + uuid + ".mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("Identify() returned %d candidates, want 2 (one per provider, never merged): %+v", len(candidates), candidates)
	}
	if _, ok := candidateBySource(candidates, "stashdb"); !ok {
		t.Errorf("missing stashdb candidate: %+v", candidates)
	}
	if _, ok := candidateBySource(candidates, "tpdb"); !ok {
		t.Errorf("missing tpdb candidate: %+v", candidates)
	}
}

func TestIdentifier_Identify_DirectID_NoUUIDFallsThrough(t *testing.T) {
	stashDB := newFakeStashDB()
	tpdb := newFakeThePornDB()
	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})

	path := "/scenes/no-id-here.mp4"
	fp := domain.Fingerprint{Metadata: map[string]any{"os_hash": "deadbeef"}}
	if _, err := id.Identify(context.Background(), fp, []string{path}, path, "/scenes"); err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if stashDB.findScenesByFingerprintsCalls != 1 {
		t.Errorf("expected fingerprint tier to run when no UUID is present, findScenesByFingerprintsCalls = %d", stashDB.findScenesByFingerprintsCalls)
	}
}

func TestIdentifier_Identify_Fingerprint_SameSceneNeverMergedAcrossProviders(t *testing.T) {
	stashDB := newFakeStashDB()
	stashDB.scenesByFingerprint = []ports.Scene{
		{ID: "stash-1", Title: "Same Real Scene", Fingerprints: []ports.Fingerprint{
			{Hash: "aa11", Algorithm: ports.FingerprintAlgorithmOSHash, Submissions: 3},
		}},
	}
	tpdb := newFakeThePornDB()
	tpdb.scenesByHash["bb22"] = ports.TPDBScene{ID: "tpdb-1", Title: "Same Real Scene", Hashes: []ports.TPDBHash{
		{Hash: "bb22", Type: "phash", Submissions: 5},
	}}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	fp := domain.Fingerprint{Metadata: map[string]any{"os_hash": "aa11", "phash": "bb22"}}
	path := "/scenes/clip.mp4"

	candidates, err := id.Identify(context.Background(), fp, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("Identify() returned %d candidates, want 2 (same real scene, two providers, never collapsed): %+v", len(candidates), candidates)
	}

	stashC, ok := candidateBySource(candidates, "stashdb")
	if !ok || stashC.ExternalRef != "stash-1" || stashC.Tier != domain.MatchTierUniqueID {
		t.Errorf("stashdb candidate = %+v, ok=%v", stashC, ok)
	}
	if got := stashC.Signals["fingerprint_submissions"]; got != 3 {
		t.Errorf("stashdb fingerprint_submissions = %v, want 3", got)
	}

	tpdbC, ok := candidateBySource(candidates, "tpdb")
	if !ok || tpdbC.ExternalRef != "tpdb-1" || tpdbC.Tier != domain.MatchTierUniqueID {
		t.Errorf("tpdb candidate = %+v, ok=%v", tpdbC, ok)
	}
	if got := tpdbC.Signals["fingerprint_submissions"]; got != 5 {
		t.Errorf("tpdb fingerprint_submissions = %v, want 5", got)
	}
}

func TestIdentifier_Identify_Fingerprint_TPDBDedupsAcrossHashes(t *testing.T) {
	tpdb := newFakeThePornDB()
	scene := ports.TPDBScene{ID: "tpdb-1", Title: "One Scene, Two Hashes", Hashes: []ports.TPDBHash{
		{Hash: "aa11", Submissions: 2},
		{Hash: "bb22", Submissions: 4},
	}}
	tpdb.scenesByHash["aa11"] = scene
	tpdb.scenesByHash["bb22"] = scene

	id := afterdark.NewIdentifier(newFakeStashDB(), tpdb, afterdark.FilenameParser{})
	fp := domain.Fingerprint{Metadata: map[string]any{"os_hash": "aa11", "phash": "bb22"}}
	path := "/scenes/clip.mp4"

	candidates, err := id.Identify(context.Background(), fp, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("Identify() returned %d candidates, want 1 (same tpdb scene via both hashes, deduped): %+v", len(candidates), candidates)
	}
	if got := candidates[0].Signals["fingerprint_submissions"]; got != 6 {
		t.Errorf("fingerprint_submissions = %v, want 6 (both hash entries summed)", got)
	}
}

func TestIdentifier_Identify_JAVCodeTier_ThePornDBOnly(t *testing.T) {
	tpdb := newFakeThePornDB()
	tpdb.javByCode["SSIS-001"] = []ports.TPDBScene{
		{ID: "jav-1", Title: "Best Match"},
		{ID: "jav-2", Title: "Tolerant Second Match"},
	}
	stashDB := newFakeStashDB()

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	path := "/jav/SSIS-001.mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/jav")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}

	var javCandidates []domain.MatchCandidate
	for _, c := range candidates {
		if c.Tier == domain.MatchTierAcoustic {
			javCandidates = append(javCandidates, c)
		}
	}
	if len(javCandidates) != 2 {
		t.Fatalf("got %d jav-code-tier candidates, want 2 (ThePornDB's own order, unfiltered): %+v", len(javCandidates), candidates)
	}
	if javCandidates[0].ExternalRef != "jav-1" || javCandidates[1].ExternalRef != "jav-2" {
		t.Errorf("jav-code candidates reordered: %+v", javCandidates)
	}
	for _, c := range javCandidates {
		if source, _ := c.Metadata["source"].(string); source != "tpdb" {
			t.Errorf("jav candidate source = %v, want tpdb", c.Metadata["source"])
		}
		if code, _ := c.Metadata["jav_code"].(string); code != "SSIS-001" {
			t.Errorf("jav candidate jav_code = %v, want SSIS-001", c.Metadata["jav_code"])
		}
	}

	// StashDB has no JAV-code equivalent — a stashdb candidate (the fuzzy
	// tier may legitimately produce one from the parsed title/studio) must
	// never carry jav_code metadata, which only the JAV-code tier sets.
	for _, c := range candidates {
		if source, _ := c.Metadata["source"].(string); source == "stashdb" {
			if _, has := c.Metadata["jav_code"]; has {
				t.Errorf("stashdb candidate unexpectedly carries jav_code: %+v", c)
			}
		}
	}
}

func TestIdentifier_Identify_FuzzyTier_BothProviders(t *testing.T) {
	stashDB := newFakeStashDB()
	stashDB.scenesByTerm["Sneaking In"] = []ports.Scene{
		{ID: "stash-fuzzy", Title: "Sneaking In", Studio: &ports.Studio{Name: "Brazzers"}},
	}
	tpdb := newFakeThePornDB()
	tpdb.scenesByTerm["Sneaking In"] = []ports.TPDBScene{
		{ID: "tpdb-fuzzy", Title: "Sneaking Into My Roommate", Site: &ports.TPDBSite{Name: "Brazzers Network"}},
	}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	path := "/scenes/Brazzers - Sneaking In.mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}

	stashC, ok := candidateBySource(candidates, "stashdb")
	if !ok || stashC.Tier != domain.MatchTierFuzzy {
		t.Fatalf("stashdb fuzzy candidate = %+v, ok=%v", stashC, ok)
	}
	if got := stashC.Signals["title_similarity"]; got != 1 {
		t.Errorf("stashdb title_similarity = %v, want 1 (exact match)", got)
	}
	if got := stashC.Signals["studio_similarity"]; got != 1 {
		t.Errorf("stashdb studio_similarity = %v, want 1 (exact match)", got)
	}

	tpdbC, ok := candidateBySource(candidates, "tpdb")
	if !ok || tpdbC.Tier != domain.MatchTierFuzzy {
		t.Fatalf("tpdb fuzzy candidate = %+v, ok=%v", tpdbC, ok)
	}
	if got := tpdbC.Signals["title_similarity"]; got <= 0 || got >= 1 {
		t.Errorf("tpdb title_similarity = %v, want a partial (0,1) match", got)
	}
}

func TestIdentifier_Identify_NoSignalNoCandidates(t *testing.T) {
	id := afterdark.NewIdentifier(newFakeStashDB(), newFakeThePornDB(), afterdark.FilenameParser{})
	path := "/scenes/unsorted/12345.mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("Identify() = %+v, want no candidates", candidates)
	}
}

func TestIdentifier_Identify_ProviderErrorIsNotFatal(t *testing.T) {
	stashDB := newFakeStashDB()
	stashDB.findScenesByFingerprintsErr = errors.New("stashdb unreachable")
	tpdb := newFakeThePornDB()
	tpdb.scenesByHash["bb22"] = ports.TPDBScene{ID: "tpdb-1", Title: "Only ThePornDB Answered"}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	fp := domain.Fingerprint{Metadata: map[string]any{"phash": "bb22"}}
	path := "/scenes/clip.mp4"

	candidates, err := id.Identify(context.Background(), fp, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v, want nil — a real provider error must be tolerated, not fatal (ADR-0027 provider independence)", err)
	}
	if len(candidates) != 1 || candidates[0].Metadata["source"] != "tpdb" {
		t.Fatalf("Identify() = %+v, want exactly the tpdb candidate despite stashdb erroring", candidates)
	}
}

func TestIdentifier_Identify_FuzzyTier_ProviderErrorTolerated(t *testing.T) {
	stashDB := newFakeStashDB()
	stashDB.searchScenesErr = errors.New("stashdb unreachable")
	tpdb := newFakeThePornDB()
	tpdb.scenesByTerm["Sneaking In"] = []ports.TPDBScene{{ID: "tpdb-fuzzy", Title: "Sneaking In"}}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	path := "/scenes/Brazzers - Sneaking In.mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/scenes")
	if err != nil {
		t.Fatalf("Identify() error = %v, want nil — a real provider error must be tolerated, not fatal", err)
	}
	if _, ok := candidateBySource(candidates, "stashdb"); ok {
		t.Errorf("candidates = %+v, want no stashdb candidate (it errored)", candidates)
	}
	if _, ok := candidateBySource(candidates, "tpdb"); !ok {
		t.Errorf("candidates = %+v, want the tpdb candidate despite stashdb erroring", candidates)
	}
}

func TestIdentifier_Identify_JAVCodeTier_ProviderErrorTolerated(t *testing.T) {
	tpdb := newFakeThePornDB()
	tpdb.resolveJAVCodeErr = errors.New("theporndb unreachable")

	id := afterdark.NewIdentifier(newFakeStashDB(), tpdb, afterdark.FilenameParser{})
	path := "/jav/SSIS-001.mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/jav")
	if err != nil {
		t.Fatalf("Identify() error = %v, want nil — a real provider error must be tolerated, not fatal", err)
	}
	for _, c := range candidates {
		if c.Tier == domain.MatchTierAcoustic {
			t.Errorf("candidates = %+v, want no jav-code-tier candidate when ResolveJAVCode errors", candidates)
		}
	}
}

func TestIdentifier_Identify_FuzzyTier_NoStudioOnEitherSideOmitsSignal(t *testing.T) {
	stashDB := newFakeStashDB()
	stashDB.scenesByTerm["Only A Title"] = []ports.Scene{{ID: "stash-fuzzy", Title: "Only A Title"}}
	tpdb := newFakeThePornDB()
	tpdb.scenesByTerm["Only A Title"] = []ports.TPDBScene{{ID: "tpdb-fuzzy", Title: "Only A Title"}}

	id := afterdark.NewIdentifier(stashDB, tpdb, afterdark.FilenameParser{})
	path := "/scenes/unsorted-root/Only A Title.mp4"

	candidates, err := id.Identify(context.Background(), domain.Fingerprint{Metadata: map[string]any{}}, []string{path}, path, "/scenes/unsorted-root")
	if err != nil {
		t.Fatalf("Identify() error = %v", err)
	}

	stashC, ok := candidateBySource(candidates, "stashdb")
	if !ok {
		t.Fatalf("candidates = %+v, want a stashdb fuzzy candidate", candidates)
	}
	if _, has := stashC.Signals["studio_similarity"]; has {
		t.Errorf("Signals = %+v, want no studio_similarity when neither side has a studio", stashC.Signals)
	}
	if got := stashC.Signals["title_similarity"]; got != 1 {
		t.Errorf("title_similarity = %v, want 1", got)
	}
}
