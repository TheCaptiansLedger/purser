//go:build integration

package fingerprint_test

import (
	"context"
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
