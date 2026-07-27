package music_test

import (
	"context"
	"os/exec"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"testing"
)

// requireFfprobe skips the test if the real ffprobe binary isn't on PATH —
// Fingerprint shells out to it directly (see docs/adr/0003-go-testing-
// standards.md's treatment of a local, fast, non-network binary: no `live`
// build-tag gating, but CI/dev environments without the media toolchain
// installed shouldn't fail the whole suite).
func requireFfprobe(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found on PATH, skipping")
	}
}

func TestFileFingerprinter_ContentTypes(t *testing.T) {
	f := music.New()
	got := f.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestFileFingerprinter_Fingerprint_FullyTaggedFLAC(t *testing.T) {
	requireFfprobe(t)
	f := music.New()

	got, err := f.Fingerprint(context.Background(), "testdata/tagged.flac", 1)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}

	wantTags := map[string]string{
		"ALBUM":                      "Test Album",
		"ALBUMARTIST":                "Test Artist",
		"DATE":                       "2001-02-03",
		"LABEL":                      "Test Label",
		"CATALOGNUMBER":              "CAT-001",
		"BARCODE":                    "0123456789012",
		"ISRC":                       "USRC17607839",
		"TITLE":                      "Side A, Track 1",
		"MUSICBRAINZ_ALBUMID":        "11111111-1111-1111-1111-111111111111",
		"MUSICBRAINZ_RELEASEGROUPID": "22222222-2222-2222-2222-222222222222",
		"MUSICBRAINZ_TRACKID":        "33333333-3333-3333-3333-333333333333",
	}
	for k, want := range wantTags {
		if got.Tags[k] != want {
			t.Errorf("Tags[%q] = %q, want %q", k, got.Tags[k], want)
		}
	}

	// The file's embedded DISCNUMBER (2) disagrees with the folder guess
	// (1) passed in — the tag must win.
	if discNumber, _ := got.Metadata["disc_number"].(int); discNumber != 2 {
		t.Errorf("Metadata[disc_number] = %v, want 2 (tag overrides folder guess)", got.Metadata["disc_number"])
	}

	// TRACKNUMBER is vinyl-style ("A1") — must survive as the exact raw
	// string, never coerced to or rejected as an int.
	if trackNumber, _ := got.Metadata["track_number"].(string); trackNumber != "A1" {
		t.Errorf("Metadata[track_number] = %v, want \"A1\"", got.Metadata["track_number"])
	}

	duration, ok := got.Metadata["duration_seconds"].(float64)
	if !ok || duration <= 0 {
		t.Errorf("Metadata[duration_seconds] = %v, want a positive float64", got.Metadata["duration_seconds"])
	}
}

func TestFileFingerprinter_Fingerprint_BlankTagsFallsBackToGuess(t *testing.T) {
	requireFfprobe(t)
	f := music.New()

	got, err := f.Fingerprint(context.Background(), "testdata/blank.flac", 3)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}

	for k, v := range got.Tags {
		t.Errorf("Tags[%q] = %q, want no tags extracted from a blank file", k, v)
	}
	if discNumber, _ := got.Metadata["disc_number"].(int); discNumber != 3 {
		t.Errorf("Metadata[disc_number] = %v, want 3 (folder guess, no DISCNUMBER tag present)", got.Metadata["disc_number"])
	}
	if trackNumber, _ := got.Metadata["track_number"].(string); trackNumber != "" {
		t.Errorf("Metadata[track_number] = %q, want empty string", trackNumber)
	}
}

func TestFileFingerprinter_Fingerprint_MP3TagAliases(t *testing.T) {
	requireFfprobe(t)
	f := music.New()

	// tagged.mp3 exercises ID3v2's normalization: ALBUM/DATE/TITLE come
	// back lowercased by ffprobe (standard frames) while
	// ALBUMARTIST/DISCNUMBER/TRACKNUMBER/MUSICBRAINZ_* stay uppercase
	// (custom TXXX frames) — the opposite pattern from FLAC. Both must
	// resolve to the same canonical Tags keys.
	got, err := f.Fingerprint(context.Background(), "testdata/tagged.mp3", 0)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if got.Tags["ALBUM"] != "Test Album" {
		t.Errorf("Tags[ALBUM] = %q, want %q", got.Tags["ALBUM"], "Test Album")
	}
	if got.Tags["ALBUMARTIST"] != "Test Artist" {
		t.Errorf("Tags[ALBUMARTIST] = %q, want %q", got.Tags["ALBUMARTIST"], "Test Artist")
	}
	if discNumber, _ := got.Metadata["disc_number"].(int); discNumber != 1 {
		t.Errorf("Metadata[disc_number] = %v, want 1", got.Metadata["disc_number"])
	}
	if trackNumber, _ := got.Metadata["track_number"].(string); trackNumber != "2" {
		t.Errorf("Metadata[track_number] = %q, want \"2\"", trackNumber)
	}
}

func TestFileFingerprinter_Fingerprint_MissingFileErrors(t *testing.T) {
	requireFfprobe(t)
	f := music.New()

	_, err := f.Fingerprint(context.Background(), "testdata/does-not-exist.flac", 0)
	if err == nil {
		t.Fatal("Fingerprint returned nil error for a nonexistent file")
	}
}

func fp(disc int, track, title string, duration float64, isrc, mbid string) domain.Fingerprint {
	return domain.Fingerprint{
		Tags: map[string]string{"TITLE": title, "ISRC": isrc, "MUSICBRAINZ_ALBUMID": mbid},
		Metadata: map[string]any{
			"disc_number":      disc,
			"track_number":     track,
			"duration_seconds": duration,
		},
	}
}

func fpWithReleaseGroup(disc int, track, title string, rgMBID string) domain.Fingerprint {
	return domain.Fingerprint{
		Tags: map[string]string{"TITLE": title, "MUSICBRAINZ_RELEASEGROUPID": rgMBID},
		Metadata: map[string]any{
			"disc_number":  disc,
			"track_number": track,
		},
	}
}

func TestFileFingerprinter_Consensus_MajorityVoteIgnoresSingleOutlier(t *testing.T) {
	f := music.New()

	fingerprints := make([]domain.Fingerprint, 0, 10)
	for i := 0; i < 9; i++ {
		fingerprints = append(fingerprints, domain.Fingerprint{
			Tags:     map[string]string{"ALBUM": "Correct Album"},
			Metadata: map[string]any{"disc_number": 1, "track_number": "1"},
		})
	}
	fingerprints = append(fingerprints, domain.Fingerprint{
		Tags:     map[string]string{"ALBUM": "Typo'd Albvm"},
		Metadata: map[string]any{"disc_number": 1, "track_number": "1"},
	})

	got, err := f.Consensus(context.Background(), fingerprints)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if got.Tags["ALBUM"] != "Correct Album" {
		t.Fatalf("Tags[ALBUM] = %q, want %q (majority over a single outlier)", got.Tags["ALBUM"], "Correct Album")
	}
	if trackCount, _ := got.Metadata["track_count"].(int); trackCount != 10 {
		t.Errorf("Metadata[track_count] = %v, want 10", got.Metadata["track_count"])
	}
}

func TestFileFingerprinter_Consensus_ConflictingMBIDsKeptAsSet(t *testing.T) {
	f := music.New()

	fingerprints := []domain.Fingerprint{
		fp(1, "1", "Track One", 180.0, "", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		fp(1, "2", "Track Two", 200.0, "", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
	}

	got, err := f.Consensus(context.Background(), fingerprints)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	mbids, ok := got.Metadata["embedded_release_mbids"].([]string)
	if !ok || len(mbids) != 2 {
		t.Fatalf("Metadata[embedded_release_mbids] = %v, want both distinct MBIDs kept, not collapsed", got.Metadata["embedded_release_mbids"])
	}
}

func TestFileFingerprinter_Consensus_ReleaseGroupMBIDsKeptAsSet(t *testing.T) {
	f := music.New()

	fingerprints := []domain.Fingerprint{
		fpWithReleaseGroup(1, "1", "Track One", "cccccccc-cccc-cccc-cccc-cccccccccccc"),
		fpWithReleaseGroup(1, "2", "Track Two", "cccccccc-cccc-cccc-cccc-cccccccccccc"),
	}

	got, err := f.Consensus(context.Background(), fingerprints)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	rgMBIDs, ok := got.Metadata["embedded_releasegroup_mbids"].([]string)
	if !ok || len(rgMBIDs) != 1 || rgMBIDs[0] != "cccccccc-cccc-cccc-cccc-cccccccccccc" {
		t.Fatalf("Metadata[embedded_releasegroup_mbids] = %v, want one agreeing MBID", got.Metadata["embedded_releasegroup_mbids"])
	}
}

func TestFileFingerprinter_Consensus_OrdersTracksByDiscThenTrackNumber(t *testing.T) {
	f := music.New()

	fingerprints := []domain.Fingerprint{
		fp(1, "10", "Disc1 Track10", 10, "", ""),
		fp(1, "2", "Disc1 Track2", 2, "", ""),
		fp(2, "1", "Disc2 Track1", 1, "", ""),
		fp(1, "1", "Disc1 Track1", 1, "", ""),
	}

	got, err := f.Consensus(context.Background(), fingerprints)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	titles, ok := got.Metadata["track_titles"].([]string)
	if !ok {
		t.Fatalf("Metadata[track_titles] = %v, want []string", got.Metadata["track_titles"])
	}
	want := []string{"Disc1 Track1", "Disc1 Track2", "Disc1 Track10", "Disc2 Track1"}
	if len(titles) != len(want) {
		t.Fatalf("track_titles = %v, want %v", titles, want)
	}
	for i, w := range want {
		if titles[i] != w {
			t.Errorf("track_titles[%d] = %q, want %q", i, titles[i], w)
		}
	}
	if discCount, _ := got.Metadata["disc_count"].(int); discCount != 2 {
		t.Errorf("Metadata[disc_count] = %v, want 2", got.Metadata["disc_count"])
	}
}

func TestFileFingerprinter_Consensus_OrdersVinylSideLetteredTracks(t *testing.T) {
	f := music.New()

	fingerprints := []domain.Fingerprint{
		fp(0, "B2", "B2", 0, "", ""),
		fp(0, "A1", "A1", 0, "", ""),
		fp(0, "B1", "B1", 0, "", ""),
		fp(0, "A2", "A2", 0, "", ""),
	}

	got, err := f.Consensus(context.Background(), fingerprints)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	titles, _ := got.Metadata["track_titles"].([]string)
	want := []string{"A1", "A2", "B1", "B2"}
	if len(titles) != len(want) {
		t.Fatalf("track_titles = %v, want %v", titles, want)
	}
	for i, w := range want {
		if titles[i] != w {
			t.Errorf("track_titles[%d] = %q, want %q", i, titles[i], w)
		}
	}
}

func TestFileFingerprinter_Consensus_EmptyGroup(t *testing.T) {
	f := music.New()

	got, err := f.Consensus(context.Background(), nil)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if trackCount, _ := got.Metadata["track_count"].(int); trackCount != 0 {
		t.Errorf("Metadata[track_count] = %v, want 0", got.Metadata["track_count"])
	}
	if _, ok := got.Metadata["embedded_release_mbids"]; ok {
		t.Errorf("Metadata[embedded_release_mbids] = %v, want unset for an empty group", got.Metadata["embedded_release_mbids"])
	}
}
