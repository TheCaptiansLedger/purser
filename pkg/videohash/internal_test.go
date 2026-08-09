package videohash

// White-box tests for the pure, ffmpeg-independent logic PHash is built
// from — frame-offset math, montage assembly, and ffmpeg argument
// construction. Deliberately kept out of videohash_test.go (the external
// black-box suite) so these run without the ffmpeg binary, per issue #556's
// verification checklist: minimize what actually requires the real
// external decoder.

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"
)

func TestFrameOffsets(t *testing.T) {
	cases := []struct {
		name     string
		duration time.Duration
	}{
		{"short", 10 * time.Second},
		{"typical scene length", 25 * time.Minute},
		{"sub-second", 500 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offsets := frameOffsets(tc.duration)
			if len(offsets) != frameCount {
				t.Fatalf("frameOffsets(%v) returned %d offsets, want %d", tc.duration, len(offsets), frameCount)
			}

			total := tc.duration.Seconds()
			wantFirst := edgeTrimFraction * total
			if offsets[0] != wantFirst {
				t.Errorf("offsets[0] = %v, want %v (edgeTrimFraction of total)", offsets[0], wantFirst)
			}

			// Evenly spaced: every consecutive gap is the same (within
			// float64 rounding), and every offset falls strictly before the
			// trailing edge trim.
			const epsilon = 1e-9
			step := offsets[1] - offsets[0]
			for i := 1; i < len(offsets); i++ {
				gotStep := offsets[i] - offsets[i-1]
				if diff := gotStep - step; diff > epsilon || diff < -epsilon {
					t.Errorf("offsets[%d]-offsets[%d] = %v, want constant step %v", i, i-1, gotStep, step)
				}
				if offsets[i] >= total-edgeTrimFraction*total+step {
					t.Errorf("offsets[%d] = %v exceeds the trailing edge trim boundary for total %v", i, offsets[i], total)
				}
			}
		})
	}
}

func TestFfmpegArgs(t *testing.T) {
	cases := []struct {
		name     string
		slowSeek bool
	}{
		{"fast seek: -ss before -i", false},
		{"slow seek: -ss after -i", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := ffmpegArgs("/videos/scene.mp4", 12.5, tc.slowSeek)

			iIdx := indexOf(args, "-i")
			ssIdx := indexOf(args, "-ss")
			if iIdx == -1 || ssIdx == -1 {
				t.Fatalf("ffmpegArgs missing -i or -ss: %v", args)
			}
			if tc.slowSeek && ssIdx < iIdx {
				t.Errorf("slowSeek=true: -ss (%d) should come after -i (%d): %v", ssIdx, iIdx, args)
			}
			if !tc.slowSeek && ssIdx > iIdx {
				t.Errorf("slowSeek=false: -ss (%d) should come before -i (%d): %v", ssIdx, iIdx, args)
			}

			if args[iIdx+1] != "/videos/scene.mp4" {
				t.Errorf("-i argument = %q, want the input path", args[iIdx+1])
			}
			if want := "12.500000"; args[ssIdx+1] != want {
				t.Errorf("-ss argument = %q, want %q", args[ssIdx+1], want)
			}

			joined := strings.Join(args, " ")
			for _, want := range []string{"-frames:v 1", "scale=160:-2", "-c:v bmp", "-f rawvideo"} {
				if !strings.Contains(joined, want) {
					t.Errorf("ffmpegArgs() = %q, want it to contain %q", joined, want)
				}
			}
		})
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func TestCombineAndHash_EmptyFrames(t *testing.T) {
	if _, err := combineAndHash(nil); err == nil {
		t.Fatal("combineAndHash(nil): want error, got nil")
	}
}

func TestCombineAndHash_Deterministic(t *testing.T) {
	frames := patternedFrames()

	got, err := combineAndHash(frames)
	if err != nil {
		t.Fatalf("combineAndHash: %v", err)
	}
	again, err := combineAndHash(frames)
	if err != nil {
		t.Fatalf("combineAndHash (second run): %v", err)
	}
	if got != again {
		t.Errorf("combineAndHash not deterministic: %d != %d", got, again)
	}
}

func TestCombineAndHash_DifferentContentDifferentHash(t *testing.T) {
	checkerboard, err := combineAndHash(patternedFrames())
	if err != nil {
		t.Fatalf("combineAndHash(checkerboard): %v", err)
	}

	gradient := make([]image.Image, frameCount)
	for i := range gradient {
		gradient[i] = gradientFrame(20, 20, i)
	}
	gradientHash, err := combineAndHash(gradient)
	if err != nil {
		t.Fatalf("combineAndHash(gradient): %v", err)
	}

	if checkerboard == gradientHash {
		t.Errorf("combineAndHash produced the same hash for visually distinct montages: %d", checkerboard)
	}
}

// patternedFrames returns frameCount deterministic checkerboard images —
// enough visual texture for goimagehash's DCT to produce a non-degenerate
// hash, unlike a flat solid color.
func patternedFrames() []image.Image {
	frames := make([]image.Image, frameCount)
	for i := range frames {
		frames[i] = checkerboardFrame(20, 20)
	}
	return frames
}

func checkerboardFrame(width, height int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			} else {
				img.Set(x, y, color.NRGBA{A: 255})
			}
		}
	}
	return img
}

// gradientFrame returns a diagonal-gradient image, seeded by i so the
// montage built from frameCount of these carries real, varying texture
// distinct from checkerboardFrame's.
func gradientFrame(width, height, i int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v := uint8((x + y + i*7) % 256) //nolint:gosec // G115: deliberate 8-bit wraparound for a synthetic test gradient, not a security-relevant conversion
			img.Set(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}
