//go:build integration

package fingerprint_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"purser/internal/app/errs"
	"purser/internal/ports"
)

// realFS implements ports.FileSystem for integration tests.
// Only OSHash is fully functional; the other methods are stubs.
type realFS struct{}

func (r *realFS) OSHash(_ context.Context, path string) (string, error) {
	return computeOSHash(path)
}

func (r *realFS) Stat(_ context.Context, _ string) (*ports.FileInfo, error) {
	return nil, errs.ErrNotFound
}

func (r *realFS) Move(_ context.Context, _, _ string) error { return nil }

func (r *realFS) Walk(_ context.Context, _ string, _ func(ports.FileInfo) error) error {
	return nil
}

// computeOSHash implements the Open Subtitles hash algorithm.
// hash = file_size + sum of 8-byte little-endian words in first and last 64 KB.
func computeOSHash(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is a test fixture
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	const chunkSize = 64 * 1024
	fileSize := info.Size()
	var hash uint64 = uint64(fileSize) //nolint:gosec // size intentionally cast to uint64

	buf := make([]byte, chunkSize)
	n, _ := io.ReadFull(f, buf)
	for i := 0; i+8 <= n; i += 8 {
		hash += binary.LittleEndian.Uint64(buf[i : i+8])
	}

	if fileSize >= chunkSize {
		if _, err := f.Seek(-chunkSize, io.SeekEnd); err != nil {
			return "", err
		}
		n, _ = io.ReadFull(f, buf)
		for i := 0; i+8 <= n; i += 8 {
			hash += binary.LittleEndian.Uint64(buf[i : i+8])
		}
	}

	return fmt.Sprintf("%016x", hash), nil
}

// tagLibFile returns the absolute path to a file in dhowden/tag's testdata/with_tags directory.
// The module is a direct dependency of this project, so the cache entry is always present.
func tagLibFile(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Skipf("go env GOMODCACHE: %v", err)
	}
	p := filepath.Join(
		strings.TrimSpace(string(out)),
		"github.com", "dhowden", "tag@v0.0.0-20240417053706-3d75831295e8",
		"testdata", "with_tags", name,
	)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("dhowden/tag fixture not found — run: go mod download: %s", p)
	}
	return p
}

// downloadFixture fetches url to testdata/name, caching on disk after the first download.
// Skips the test if the download fails (no network, server unavailable, etc.).
func downloadFixture(t *testing.T, name, url string) string {
	t.Helper()
	dst := filepath.Join("testdata", name)
	if _, err := os.Stat(dst); err == nil {
		return dst
	}
	t.Logf("downloading fixture %s", url)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url) //nolint:gosec // URL is a hardcoded test fixture constant
	if err != nil {
		t.Skipf("cannot download fixture (no network?): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("cannot download fixture: HTTP %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dst) //nolint:gosec // dst path constructed from test constant
	if err != nil {
		t.Fatal("create fixture:", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(dst)
		t.Fatal("write fixture:", err)
	}
	_ = f.Close()
	return dst
}

// generateMP4 creates a 2-second synthetic MP4 via ffmpeg and caches it in testdata/.
// Skips the test if ffmpeg is not available.
func generateMP4(t *testing.T) string {
	t.Helper()
	dst := filepath.Join("testdata", "synthetic.mp4")
	if _, err := os.Stat(dst); err == nil {
		return dst
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available for video fixture generation")
	}
	cmd := exec.Command("ffmpeg", //nolint:gosec // intentional external tool invocation
		"-f", "lavfi", "-i", "testsrc=duration=2:size=128x96:rate=5",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264", "-c:a", "aac", "-movflags", "+faststart",
		dst, "-y", "-loglevel", "quiet",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg fixture generation failed: %v\n%s", err, out)
	}
	return dst
}

// buildTestPDF writes a minimal valid PDF-1.4 with the given Title and Author to testdata/sample.pdf.
// pdfcpu reads these values from the PDF Info dictionary.
func buildTestPDF(t *testing.T, title, author string) string {
	t.Helper()
	dst := filepath.Join("testdata", "sample.pdf")
	if _, err := os.Stat(dst); err == nil {
		return dst
	}
	data := minimalPDFBytes(title, author)
	if err := os.WriteFile(dst, data, 0o640); err != nil {
		t.Fatal("write sample.pdf:", err)
	}
	return dst
}

// minimalPDFBytes returns a valid PDF-1.4 byte slice with Title and Author in the Info dict.
func minimalPDFBytes(title, author string) []byte {
	var body strings.Builder
	offsets := make([]int, 4)

	body.WriteString("%PDF-1.4\n")

	offsets[1] = body.Len()
	body.WriteString("1 0 obj\n<</Type /Catalog /Pages 2 0 R>>\nendobj\n\n")

	offsets[2] = body.Len()
	body.WriteString("2 0 obj\n<</Type /Pages /Count 0 /Kids []}}\nendobj\n\n")

	offsets[3] = body.Len()
	fmt.Fprintf(&body, "3 0 obj\n<</Title (%s) /Author (%s)>>\nendobj\n\n", title, author)

	xrefOffset := body.Len()
	fmt.Fprintf(&body, "xref\n0 4\n0000000000 65535 f \n")
	for i := 1; i <= 3; i++ {
		fmt.Fprintf(&body, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&body, "\ntrailer\n<</Size 4 /Root 1 0 R /Info 3 0 R>>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	return []byte(body.String())
}

// readTagsWithMetaflac runs metaflac --export-tags-to=- on path and returns the tags as a
// map of uppercase key → value. Used as a ground-truth oracle in fingerprinter integration tests.
func readTagsWithMetaflac(t *testing.T, path string) map[string]string {
	t.Helper()
	out, err := exec.Command("metaflac", "--export-tags-to=-", path).Output() //nolint:gosec // path is a test fixture
	if err != nil {
		t.Fatalf("metaflac: %v", err)
	}
	tags := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		tags[strings.ToUpper(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return tags
}
