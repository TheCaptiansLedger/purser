//go:build integration

package fingerprint_test

import (
	"context"
	"os/exec"
	"strconv"
	"testing"

	"purser/internal/adapters/fingerprint"
	"purser/internal/domain"
)

// These tests use the sample files shipped with github.com/dhowden/tag (our direct dependency).
// The files were tagged with: artist="Test Artist", title="Test Title", album="Test Album",
// track=3, disc=2, year=2000. Known values allow precise assertions.

func TestMusicFingerprinter_Integration_MP3(t *testing.T) {
	path := tagLibFile(t, "sample.id3v23.mp3")

	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}

	checks := map[string]string{
		"title":  "Test Title",
		"artist": "Test Artist",
		"album":  "Test Album",
	}
	for key, want := range checks {
		if got := result.EmbeddedTags[key]; got != want {
			t.Errorf("EmbeddedTags[%q] = %q, want %q", key, got, want)
		}
	}

	if result.EmbeddedTags["track_number"] == "" {
		t.Error("expected non-empty track_number")
	}
	if result.EmbeddedTags["disc_number"] == "" {
		t.Error("expected non-empty disc_number")
	}
	if result.EmbeddedTags["date"] == "" {
		t.Error("expected non-empty date")
	}
}

func TestMusicFingerprinter_Integration_FLAC(t *testing.T) {
	path := tagLibFile(t, "sample.flac")

	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}

	checks := map[string]string{
		"title":  "Test Title",
		"artist": "Test Artist",
		"album":  "Test Album",
	}
	for key, want := range checks {
		if got := result.EmbeddedTags[key]; got != want {
			t.Errorf("EmbeddedTags[%q] = %q, want %q", key, got, want)
		}
	}
}

// TestMusicFingerprinter_Integration_TagExtractionMatchesMetaflac builds a temp FLAC
// containing all new tag types, reads the ground-truth values back via metaflac, then
// asserts the fingerprinter extracts the same values. The test is self-verifying: no
// expected values are hardcoded — metaflac is the oracle.
func TestMusicFingerprinter_Integration_TagExtractionMatchesMetaflac(t *testing.T) {
	if _, err := exec.LookPath("metaflac"); err != nil {
		t.Skip("metaflac not available")
	}

	data := buildFLACWithVorbisComment(map[string]string{
		"ISRC":        "TSTEST00001",
		"UPC":         "012345678901",
		"LABEL":       "Test Label",
		"TRACKTOTAL":  "2",
		"DISCTOTAL":   "1",
		"TRACKNUMBER": "1",
		"DISCNUMBER":  "1",
		"TITLE":       "Track 01",
		"ARTIST":      "Test Artist",
		"ALBUM":       "Test Album",
	})
	path := writeTempFile(t, ".flac", data)

	oracle := readTagsWithMetaflac(t, path)
	t.Logf("metaflac oracle: %v", oracle)

	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}
	t.Logf("embedded tags: %v", result.EmbeddedTags)

	// embeddedKey → metaflac tag name
	checks := map[string]string{
		"isrc":        "ISRC",
		"barcode":     "UPC",
		"label":       "LABEL",
		"track_total": "TRACKTOTAL",
		"disc_total":  "DISCTOTAL",
	}
	for embKey, metaKey := range checks {
		want := oracle[metaKey]
		if want == "" {
			t.Errorf("metaflac did not return tag %q — fixture may be invalid", metaKey)
			continue
		}
		if got := result.EmbeddedTags[embKey]; got != want {
			t.Errorf("EmbeddedTags[%q] = %q, metaflac %q = %q", embKey, got, metaKey, want)
		}
	}
}

// TestMusicFingerprinter_Integration_AllTracksHaveISRC creates N temp FLACs each with a
// distinct ISRC, uses metaflac to confirm the tag is present in each file, then asserts
// the fingerprinter extracts the same ISRC.
func TestMusicFingerprinter_Integration_AllTracksHaveISRC(t *testing.T) {
	if _, err := exec.LookPath("metaflac"); err != nil {
		t.Skip("metaflac not available")
	}

	isrcs := []string{"TSTEST00001", "TSTEST00002", "TSTEST00003"}
	fp := fingerprint.NewMusicFingerprinter()

	for i, isrc := range isrcs {
		trackNum := strconv.Itoa(i + 1)
		data := buildFLACWithVorbisComment(map[string]string{
			"ISRC":        isrc,
			"TRACKNUMBER": trackNum,
		})
		path := writeTempFile(t, ".flac", data)

		oracle := readTagsWithMetaflac(t, path)
		wantISRC := oracle["ISRC"]
		if wantISRC == "" {
			t.Fatalf("track %s: fixture missing ISRC tag — metaflac returned %v", trackNum, oracle)
		}

		result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
			Path:        path,
			ContentType: domain.ContentTypeMusic,
		})
		if err != nil {
			t.Fatalf("track %s: %v", trackNum, err)
		}
		if result == nil {
			t.Fatalf("track %s: expected non-nil Fingerprint", trackNum)
		}

		if got := result.EmbeddedTags["isrc"]; got != wantISRC {
			t.Errorf("track %s: EmbeddedTags[\"isrc\"] = %q, metaflac says %q", trackNum, got, wantISRC)
		}
	}
}
