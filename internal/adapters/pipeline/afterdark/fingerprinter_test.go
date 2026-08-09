package afterdark_test

import (
	"context"
	"log/slog"
	"os/exec"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/domain"
	"testing"

	"go.opentelemetry.io/otel"
)

// requireFfmpegToolchain skips the test if either real binary Fingerprint
// shells out to (ffprobe directly, ffmpeg via pkg/videohash) isn't on PATH.
// Same non-network local-binary skip convention
// internal/adapters/pipeline/music/fingerprinter_test.go and
// pkg/videohash/videohash_test.go already use, per docs/adr/0003-go-
// testing-standards.md.
func requireFfmpegToolchain(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not found on PATH, skipping")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found on PATH, skipping")
	}
}

// TestNew_AppliesOptions exercises WithLogger/WithTracerProvider — New
// applies every option before returning, and a FileFingerprinter built with
// overrides must behave identically to the default for callers.
func TestNew_AppliesOptions(t *testing.T) {
	f := afterdark.New(afterdark.WithLogger(slog.Default()), afterdark.WithTracerProvider(otel.GetTracerProvider()))
	got := f.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

func TestFileFingerprinter_ContentTypes(t *testing.T) {
	f := afterdark.New()
	got := f.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

// TestFileFingerprinter_Fingerprint_Synthetic exercises the full
// probe+OSHash+PHash pipeline against a committed synthetic fixture
// (testdata/synthetic.mp4, an ffmpeg-generated "testsrc" pattern — not a
// real scene, just deterministic bytes, same technique
// pkg/videohash/testdata/synthetic.mp4 uses). At 81362 bytes it's
// deliberately larger than pkg/videohash's own 23777-byte fixture — big
// enough to clear pkg/filehash.OSHash's 64 KiB minimum, which that smaller
// fixture doesn't.
func TestFileFingerprinter_Fingerprint_Synthetic(t *testing.T) {
	requireFfmpegToolchain(t)
	f := afterdark.New()

	got, err := f.Fingerprint(context.Background(), "testdata/synthetic.mp4", 0)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}

	if got.Tags != nil {
		t.Errorf("Tags = %v, want nil (AfterDark has no embedded-tag identification path)", got.Tags)
	}

	duration, ok := got.Metadata["duration_seconds"].(float64)
	if !ok || duration < 7.9 || duration > 8.1 {
		t.Errorf("Metadata[duration_seconds] = %v, want ~8.0", got.Metadata["duration_seconds"])
	}
	if codec, _ := got.Metadata["codec"].(string); codec != "h264" {
		t.Errorf("Metadata[codec] = %q, want %q", codec, "h264")
	}
	osHash, ok := got.Metadata["os_hash"].(string)
	if !ok || len(osHash) != 16 {
		t.Errorf("Metadata[os_hash] = %v, want a 16-character hex string", got.Metadata["os_hash"])
	}
	pHash, ok := got.Metadata["phash"].(string)
	if !ok || len(pHash) != 16 {
		t.Errorf("Metadata[phash] = %v, want a 16-character hex string", got.Metadata["phash"])
	}

	// Determinism: hashing the same file twice must produce the same
	// os_hash/phash.
	again, err := f.Fingerprint(context.Background(), "testdata/synthetic.mp4", 0)
	if err != nil {
		t.Fatalf("Fingerprint (second run) returned error: %v", err)
	}
	if again.Metadata["os_hash"] != osHash {
		t.Errorf("os_hash not deterministic: %v != %v", again.Metadata["os_hash"], osHash)
	}
	if again.Metadata["phash"] != pHash {
		t.Errorf("phash not deterministic: %v != %v", again.Metadata["phash"], pHash)
	}
}

// TestFileFingerprinter_Fingerprint_TinyFileOmitsOSHash exercises
// pkg/filehash.ErrFileTooSmall's documented non-fatal treatment:
// testdata/synthetic_tiny.mp4 (23777 bytes, well under OSHash's 64 KiB
// minimum — the identical fixture pkg/videohash/testdata/synthetic.mp4
// uses, so PHash's own frame-sampling is already proven to succeed against
// it) must still fingerprint successfully, with every other signal
// populated and only os_hash absent.
func TestFileFingerprinter_Fingerprint_TinyFileOmitsOSHash(t *testing.T) {
	requireFfmpegToolchain(t)
	f := afterdark.New()

	got, err := f.Fingerprint(context.Background(), "testdata/synthetic_tiny.mp4", 0)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}

	if _, ok := got.Metadata["os_hash"]; ok {
		t.Errorf("Metadata[os_hash] = %v, want absent for a file under the 64 KiB OSHash minimum", got.Metadata["os_hash"])
	}
	if _, ok := got.Metadata["phash"].(string); !ok {
		t.Errorf("Metadata[phash] = %v, want a string (PHash doesn't share OSHash's size floor)", got.Metadata["phash"])
	}
	duration, ok := got.Metadata["duration_seconds"].(float64)
	if !ok || duration < 5.9 || duration > 6.1 {
		t.Errorf("Metadata[duration_seconds] = %v, want ~6.0", got.Metadata["duration_seconds"])
	}
}

// TestFileFingerprinter_Fingerprint_AudioOnlyFileErrors exercises PHash's
// own error path surfacing through Fingerprint: testdata/audio_only.mp4
// probes successfully (ffprobe reports a valid format duration and an
// "aac" stream) but has no video stream at all, so pkg/videohash.PHash's
// frame extraction fails — the same real-world shape a mislabeled or
// corrupt "video" file would have.
func TestFileFingerprinter_Fingerprint_AudioOnlyFileErrors(t *testing.T) {
	requireFfmpegToolchain(t)
	f := afterdark.New()

	_, err := f.Fingerprint(context.Background(), "testdata/audio_only.mp4", 0)
	if err == nil {
		t.Fatal("Fingerprint returned nil error for an audio-only file (no video stream to perceptual-hash)")
	}
}

func TestFileFingerprinter_Fingerprint_MissingFileErrors(t *testing.T) {
	requireFfmpegToolchain(t)
	f := afterdark.New()

	_, err := f.Fingerprint(context.Background(), "testdata/does-not-exist.mp4", 0)
	if err == nil {
		t.Fatal("Fingerprint returned nil error for a nonexistent file")
	}
}

func TestFileFingerprinter_Consensus_SingleFilePassthrough(t *testing.T) {
	f := afterdark.New()

	fp := domain.Fingerprint{Metadata: map[string]any{"os_hash": "abc123", "phash": "def456"}}
	got, err := f.Consensus(context.Background(), []domain.Fingerprint{fp})
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if got.Metadata["os_hash"] != "abc123" || got.Metadata["phash"] != "def456" {
		t.Errorf("Consensus() = %+v, want the sole input Fingerprint returned unchanged", got)
	}
}

func TestFileFingerprinter_Consensus_EmptyGroup(t *testing.T) {
	f := afterdark.New()

	got, err := f.Consensus(context.Background(), nil)
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if got.Tags != nil || got.Metadata != nil {
		t.Errorf("Consensus() = %+v, want empty domain.Fingerprint for an empty group", got)
	}
}
