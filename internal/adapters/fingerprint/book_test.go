package fingerprint_test

import (
	"archive/zip"
	"bytes"
	"context"
	"purser/internal/adapters/fingerprint"
	"purser/internal/domain"
	"testing"
)

func buildEPUB(t *testing.T, title, creator, isbn string) []byte {
	t.Helper()

	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf"
         xmlns:dc="http://purl.org/dc/elements/1.1/"
         xmlns:opf="http://www.idpf.org/2007/opf"
         version="2.0">
  <metadata>
    <dc:title>` + title + `</dc:title>
    <dc:creator>` + creator + `</dc:creator>
    <dc:identifier opf:scheme="ISBN">` + isbn + `</dc:identifier>
  </metadata>
</package>`

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	f, err := w.Create("content.opf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(opf)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBookFingerprinter_ContentTypes(t *testing.T) {
	fp := fingerprint.NewBookFingerprinter()
	cts := fp.ContentTypes()
	if len(cts) != 1 || cts[0] != domain.ContentTypeBook {
		t.Errorf("ContentTypes() = %v, want [book]", cts)
	}
}

func TestBookFingerprinter_UnsupportedType(t *testing.T) {
	fp := fingerprint.NewBookFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        "/fake/file.epub",
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for non-book content type, got %+v", result)
	}
}

func TestBookFingerprinter_EPUB(t *testing.T) {
	const wantTitle = "Test Book Title"
	const wantCreator = "Test Author"
	const wantISBN = "978-3-16-148410-0"

	data := buildEPUB(t, wantTitle, wantCreator, wantISBN)
	path := writeTempFile(t, ".epub", data)

	fp := fingerprint.NewBookFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeBook,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}
	if result.EmbeddedTags["title"] != wantTitle {
		t.Errorf("title = %q, want %q", result.EmbeddedTags["title"], wantTitle)
	}
	if result.EmbeddedTags["creator"] != wantCreator {
		t.Errorf("creator = %q, want %q", result.EmbeddedTags["creator"], wantCreator)
	}
	if result.ISBN != wantISBN {
		t.Errorf("ISBN = %q, want %q", result.ISBN, wantISBN)
	}
}

func TestBookFingerprinter_UnknownExtension(t *testing.T) {
	// .mobi and similar have no extractor yet; the Fingerprint should be non-nil
	// but empty (no error, best-effort).
	path := writeTempFile(t, ".mobi", []byte("fake mobi content"))

	fp := fingerprint.NewBookFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeBook,
	})
	if err != nil {
		t.Fatalf("expected no error for unknown extension, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint (empty, not nil) for unknown extension")
	}
}
