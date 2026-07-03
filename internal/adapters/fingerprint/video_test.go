package fingerprint_test

import (
	"context"
	"purser/internal/adapters/fingerprint"
	"purser/internal/domain"
	"testing"
)

func TestVideoFingerprinter_ContentTypes(t *testing.T) {
	fp := fingerprint.NewVideoFingerprinter(&stubFS{})
	cts := fp.ContentTypes()
	want := map[domain.ContentType]bool{
		domain.ContentTypeMovie: true,
		domain.ContentTypeTV:    true,
		domain.ContentTypeAdult: true,
		domain.ContentTypeJAV:   true,
	}
	if len(cts) != len(want) {
		t.Fatalf("ContentTypes() len = %d, want %d", len(cts), len(want))
	}
	for _, ct := range cts {
		if !want[ct] {
			t.Errorf("unexpected content type: %q", ct)
		}
	}
}

func TestVideoFingerprinter_OSHash(t *testing.T) {
	const wantHash = "abc123def456abc1"
	path := writeTempFile(t, ".mp4", []byte("fake video content"))

	fp := fingerprint.NewVideoFingerprinter(&stubFS{hash: wantHash})
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMovie,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}
	if result.OSHash != wantHash {
		t.Errorf("OSHash = %q, want %q", result.OSHash, wantHash)
	}
}

func TestVideoFingerprinter_UnsupportedType(t *testing.T) {
	fp := fingerprint.NewVideoFingerprinter(&stubFS{hash: "abc"})
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        "/fake/file.mp3",
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for unsupported content type, got %+v", result)
	}
}

func TestVideoFingerprinter_AllVideoTypesSupported(t *testing.T) {
	const wantHash = "deadbeef12345678"
	path := writeTempFile(t, ".mkv", []byte("fake mkv content"))

	videoTypes := []domain.ContentType{
		domain.ContentTypeMovie,
		domain.ContentTypeTV,
		domain.ContentTypeAdult,
		domain.ContentTypeJAV,
	}
	fp := fingerprint.NewVideoFingerprinter(&stubFS{hash: wantHash})
	for _, ct := range videoTypes {
		result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
			Path:        path,
			ContentType: ct,
		})
		if err != nil {
			t.Errorf("ContentType %q: unexpected error: %v", ct, err)
			continue
		}
		if result == nil {
			t.Errorf("ContentType %q: expected non-nil Fingerprint", ct)
			continue
		}
		if result.OSHash != wantHash {
			t.Errorf("ContentType %q: OSHash = %q, want %q", ct, result.OSHash, wantHash)
		}
	}
}
