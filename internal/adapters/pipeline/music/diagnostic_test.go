package music

// This file is deliberately package music, not music_test — every other
// test file in this package is black-box (music_test), but this
// diagnostic's whole point is reporting the position-by-position detail
// title_set_overlap/duration_match are computed from
// (titleComparisons/durationComparisons — see identifier.go), which isn't
// observable through Identifier/ConfidenceScore's public API at all. A
// small handful of test-only helpers below (requireFfprobeDiag,
// loadDiagJSON) duplicate their music_test-package equivalents
// (fingerprinter_test.go's requireFfprobe, persister_test.go's
// loadTestdataJSON) rather than sharing them — package music and package
// music_test are genuinely separate packages even though they share a
// directory, so an unexported helper in one is invisible to the other.

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// requireFfprobeDiag skips the test if the real ffprobe binary isn't on
// PATH — see fingerprinter_test.go's requireFfprobe (package music_test,
// can't be called from here).
func requireFfprobeDiag(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found on PATH, skipping")
	}
}

// loadDiagJSON decodes testdata/name (a real recorded MusicBrainz
// response) into out, failing the test on any error. See
// persister_test.go's loadTestdataJSON (package music_test, can't be
// called from here).
func loadDiagJSON(t *testing.T, name string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decoding testdata/%s: %v", name, err)
	}
}

// diagnosticCase names one real, local album fixture to compare against a
// specific, known MusicBrainz release — see TestDiagnoseAlbum's own doc
// comment.
type diagnosticCase struct {
	name string

	// dir holds the real audio files, relative to this package directory
	// — gitignored, local-only (test-data/** per the repo's own "deliberate
	// exception, not part of this tree" convention), never present in CI
	// or a fresh clone. The test skips cleanly when it's absent instead of
	// failing.
	dir string

	// releaseJSON is testdata/<file> — a real recorded MusicBrainz release
	// response (musicbrainz.Client's actual inc=, not a guess — see
	// docs on LookupRelease). Empty skips this case (no fixture recorded
	// yet).
	releaseJSON string
}

// diagnosticCases is the known set of real-album-vs-real-MusicBrainz-
// release comparisons this diagnostic can run. Add a case here (and
// record its release with `curl .../release/<mbid>?fmt=json&inc=recordings+labels+release-groups+artist-credits+isrcs`
// into testdata/, matching internal/adapters/musicbrainz's own inc=) to
// diagnose a new album.
var diagnosticCases = []diagnosticCase{
	{
		name:        "hi_infidelity",
		dir:         "../../../../test-data/music/1980 Hi Infidelity",
		releaseJSON: "release_lookup_hi_infidelity.json",
	},
	{
		name:        "ronstadt_box_set",
		dir:         "../../../../test-data/music/(2014) - Linda Ronstadt - The 70's Studio Album Collection [24Bit-192kHz]",
		releaseJSON: "release_lookup_ronstadt.json",
	},
}

// fingerprintAlbum walks dir for real audio files (sidecar_classifier.go's
// own audioExtensions set — reused directly rather than re-deriving a
// filter, so this diagnostic never drifts from what the real M10a
// classifier actually treats as audio), runs the real
// FileFingerprinter (ffprobe) against every one, and reduces them to one
// Consensus Fingerprint — the exact same real production code the scan
// pipeline itself runs, so a bug in fingerprinting/consensus (like
// purser#522's mergedTagIndex collision) shows up here too, not just in
// production. discNumberGuess per file comes from matchDiscFolder against
// the file's immediate parent directory name (M3b's own real grouping
// logic — "Disc 1", "CD2", etc.), the same signal the real Grouping
// implementation derives it from.
func fingerprintAlbum(t *testing.T, dir string) domain.Fingerprint {
	t.Helper()
	f := New()
	ctx := context.Background()

	var fingerprints []domain.Fingerprint
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if _, ok := audioExtensions[filepath.Ext(path)]; !ok {
			return nil
		}
		discGuess := 0
		if n, ok := matchDiscFolder(filepath.Base(filepath.Dir(path))); ok {
			discGuess = n
		}
		fp, err := f.Fingerprint(ctx, path, discGuess)
		if err != nil {
			return err
		}
		fingerprints = append(fingerprints, fp)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	if len(fingerprints) == 0 {
		t.Skipf("found no audio files under %s (directory exists but is empty/incomplete locally)", dir)
	}

	consensus, err := f.Consensus(ctx, fingerprints)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	return consensus
}

// TestDiagnoseAlbum is an on-demand diagnostic, not a pass/fail
// regression gate: `go test ./internal/adapters/pipeline/music/... -run
// TestDiagnoseAlbum/hi_infidelity -v` to see its output. For each
// diagnosticCases entry it runs the real fingerprinting/consensus
// pipeline against dir's actual audio files, then the real
// titleComparisons/durationComparisons/trackCountSignal/nameFuzzySignal
// functions (the exact code title_set_overlap/duration_match/
// track_count_match/name_fuzzy_score are computed from — see
// identifier.go's scoreCandidate) against releaseJSON, and logs one row
// per track: our value, MusicBrainz's value, and whether it counted as a
// match. A scoring gap shows up as a specific row here, not a
// re-run-with-debug-logging investigation.
//
// name_fuzzy_score uses the consensus's own ALBUM/ARTIST tags as the name
// query — real Identify falls back to a filename-parsed artist name when
// no ARTIST tag is present (M6), which this diagnostic doesn't replicate,
// so that one signal's number here is a lower bound, not necessarily
// exactly what production would compute. title_set_overlap and
// duration_match (the two signals with real per-position detail to
// report) don't depend on that fallback at all.
func TestDiagnoseAlbum(t *testing.T) {
	requireFfprobeDiag(t)

	for _, tc := range diagnosticCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.dir); err != nil {
				t.Skipf("album directory not present locally (expected — gitignored, local-only fixture): %v", err)
			}
			if tc.releaseJSON == "" {
				t.Skip("no recorded MusicBrainz release fixture wired up for this case yet")
			}

			var release ports.Release
			loadDiagJSON(t, tc.releaseJSON, &release)

			consensus := fingerprintAlbum(t, tc.dir)
			ourTitles, _ := consensus.Metadata["track_titles"].([]string)
			candidateTracks := flattenReleaseTracks(&release)
			t.Logf("=== %s: %d of our tracks vs %d MusicBrainz tracks (%s) ===", tc.name, len(ourTitles), len(candidateTracks), release.Title)

			reportTitleComparisons(t, consensus, &release)
			reportDurationComparisons(t, consensus, &release)

			if v, ok := trackCountSignal(consensus, &release); ok {
				t.Logf("track_count_match = %.3f", v)
			}
			albumTag, artistTag := consensus.Tags["ALBUM"], consensus.Tags["ARTIST"]
			if v, ok := nameFuzzySignal(artistTag, albumTag, artistTag != "" || albumTag != "", &release); ok {
				t.Logf("name_fuzzy_score = %.3f (from ALBUM=%q ARTIST=%q — see this test's own doc comment on the filename-fallback caveat)", v, albumTag, artistTag)
			}
		})
	}
}

// reportTitleComparisons logs titleComparisons' real per-position detail
// and a full/partial/no-match tally.
func reportTitleComparisons(t *testing.T, consensus domain.Fingerprint, release *ports.Release) {
	t.Helper()
	comparisons, ok := titleComparisons(consensus, release)
	if !ok {
		t.Log("title_set_overlap: not evaluable (no track_titles on our side or no tracks on the candidate)")
		return
	}
	full, partial, none := 0, 0, 0
	for _, c := range comparisons {
		verdict := "no match"
		switch {
		case c.credit >= 1:
			full++
			verdict = "MATCH"
		case c.credit > 0:
			partial++
			verdict = "partial match"
		default:
			none++
		}
		t.Logf("  title[%2d]  ours=%-45q  musicbrainz=%-45q  similarity=%.3f  %s",
			c.position, c.ourTitle, c.candidateTitle, c.similarity, verdict)
	}
	credit := 0.0
	for _, c := range comparisons {
		credit += c.credit
	}
	t.Logf("title_set_overlap = %.3f  (%d full, %d partial, %d no-match of %d)", credit/float64(len(comparisons)), full, partial, none, len(comparisons))
}

// reportDurationComparisons logs durationComparisons' real per-position
// detail and a within/outside-tolerance tally.
func reportDurationComparisons(t *testing.T, consensus domain.Fingerprint, release *ports.Release) {
	t.Helper()
	comparisons, ok := durationComparisons(consensus, release)
	if !ok {
		t.Log("duration_match: not evaluable (no track_durations on our side or no tracks on the candidate)")
		return
	}
	within, outside, skipped := 0, 0, 0
	for _, c := range comparisons {
		var verdict string
		switch {
		case !c.evaluable:
			skipped++
			verdict = "not evaluable"
		case c.withinTolerance:
			within++
			verdict = "within tolerance"
		default:
			outside++
			verdict = "OUTSIDE tolerance"
		}
		t.Logf("  duration[%2d]  ours=%7.1fs  musicbrainz=%7.1fs  %s", c.position, c.ourSeconds, c.candidateSeconds, verdict)
	}
	t.Logf("duration_match = %.3f  (%d within, %d outside, %d not evaluable of %d)", float64(within)/float64(len(comparisons)), within, outside, skipped, len(comparisons))
}
