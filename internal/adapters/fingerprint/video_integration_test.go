//go:build integration

package fingerprint_test

import (
	"context"
	"os/exec"
	"testing"

	"purser/internal/adapters/fingerprint"
	"purser/internal/domain"
)

func TestVideoFingerprinter_Integration_OSHash(t *testing.T) {
	path := generateMP4(t)

	fp := fingerprint.NewVideoFingerprinter(&realFS{})
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
	if len(result.OSHash) != 16 {
		t.Errorf("OSHash = %q, want 16-char hex", result.OSHash)
	}
}

func TestVideoFingerprinter_Integration_PHash(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available — PHash test skipped")
	}

	path := generateMP4(t)

	fp := fingerprint.NewVideoFingerprinter(&realFS{})
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
	if len(result.PHash) != 48 {
		t.Errorf("PHash = %q (len %d), want 48-char hex string", result.PHash, len(result.PHash))
	}
}
