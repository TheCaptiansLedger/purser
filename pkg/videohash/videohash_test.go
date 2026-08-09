package videohash_test

import (
	"context"
	"errors"
	"os/exec"
	"purser/pkg/videohash"
	"testing"
	"time"
)

// requireFFmpeg skips the test if the real ffmpeg binary isn't on PATH —
// PHash shells out to it directly. Same non-network local-binary skip
// convention internal/adapters/pipeline/music/fingerprinter_test.go already
// uses for ffprobe, per docs/adr/0003-go-testing-standards.md.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found on PATH, skipping")
	}
}

func TestPHash_InvalidDuration(t *testing.T) {
	cases := []time.Duration{0, -1 * time.Second}
	for _, d := range cases {
		if _, err := videohash.PHash(context.Background(), "irrelevant.mp4", d); !errors.Is(err, videohash.ErrInvalidDuration) {
			t.Errorf("PHash(duration=%v) error = %v, want ErrInvalidDuration", d, err)
		}
	}
}

// TestPHash_Synthetic exercises the full extraction+hash pipeline against a
// tiny committed synthetic fixture (testdata/synthetic.mp4, an
// ffmpeg-generated "testsrc" pattern — not a real scene, just deterministic
// bytes). This is the one part of this package that genuinely can't be
// tested without a real video decoder: ffmpeg itself. Flagged per issue
// #556's own verification checklist — unavoidable, not skipped by
// oversight. Every other code path (frame-offset math, montage assembly,
// hashing, ffmpeg argument construction) is covered without ffmpeg in
// internal_test.go.
func TestPHash_Synthetic(t *testing.T) {
	requireFFmpeg(t)

	got, err := videohash.PHash(context.Background(), "testdata/synthetic.mp4", 6*time.Second)
	if err != nil {
		t.Fatalf("PHash: %v", err)
	}

	// Determinism: hashing the same file twice must produce the same value.
	again, err := videohash.PHash(context.Background(), "testdata/synthetic.mp4", 6*time.Second)
	if err != nil {
		t.Fatalf("PHash (second run): %v", err)
	}
	if got != again {
		t.Errorf("PHash not deterministic: %s != %s", videohash.String(got), videohash.String(again))
	}

	if s := videohash.String(got); len(s) != 16 {
		t.Errorf("String(%d) = %q, want 16 hex characters", got, s)
	}
}

func TestPHash_NonexistentFile(t *testing.T) {
	requireFFmpeg(t)

	if _, err := videohash.PHash(context.Background(), "testdata/does-not-exist.mp4", time.Second); err == nil {
		t.Fatal("PHash: want error for nonexistent file, got nil")
	}
}

func TestPHash_ContextCanceled(t *testing.T) {
	requireFFmpeg(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := videohash.PHash(ctx, "testdata/synthetic.mp4", 6*time.Second); err == nil {
		t.Fatal("PHash: want error for canceled context, got nil")
	}
}

func TestDistance(t *testing.T) {
	cases := []struct {
		name string
		a, b uint64
		want int
	}{
		{"identical", 0xff00ff00, 0xff00ff00, 0},
		{"one bit differs", 0b0001, 0b0000, 1},
		{"all 64 bits differ", 0, ^uint64(0), 64},
		{"order independent", 0b1010, 0b0101, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := videohash.Distance(tc.a, tc.b); got != tc.want {
				t.Errorf("Distance(%#x, %#x) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
			// Distance is symmetric.
			if got := videohash.Distance(tc.b, tc.a); got != tc.want {
				t.Errorf("Distance(%#x, %#x) = %d, want %d", tc.b, tc.a, got, tc.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		name string
		h    uint64
		want string
	}{
		{"zero, zero-padded", 0, "0000000000000000"},
		{"known value", 0xe18951f0e3a49edc, "e18951f0e3a49edc"},
		{"max value", ^uint64(0), "ffffffffffffffff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := videohash.String(tc.h); got != tc.want {
				t.Errorf("String(%#x) = %q, want %q", tc.h, got, tc.want)
			}
		})
	}
}
