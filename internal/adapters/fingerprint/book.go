package fingerprint

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"strings"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
)

var bookContentTypes = []domain.ContentType{domain.ContentTypeBook}

type bookFingerprinter struct{}

var _ ports.FileFingerprinter = (*bookFingerprinter)(nil)

// NewBookFingerprinter returns a FileFingerprinter for book content (EPUB and PDF).
func NewBookFingerprinter() ports.FileFingerprinter {
	return &bookFingerprinter{}
}

func (b *bookFingerprinter) ContentTypes() []domain.ContentType {
	return bookContentTypes
}

func (b *bookFingerprinter) Fingerprint(ctx context.Context, f domain.ScannedFile) (*domain.Fingerprint, error) {
	if !slices.Contains(b.ContentTypes(), f.ContentType) {
		return nil, nil //nolint:nilnil // port contract: unsupported type returns nil, nil
	}

	fp := &domain.Fingerprint{EmbeddedTags: make(map[string]string)}

	ext := strings.ToLower(filepath.Ext(f.Path))
	var err error
	switch ext {
	case ".epub":
		err = b.readEPUB(f.Path, fp)
	case ".pdf":
		err = b.readPDF(f.Path, fp)
	}
	if err != nil {
		slog.WarnContext(ctx, "book metadata read skipped", "path", f.Path, "err", err)
	}

	return fp, nil
}

func (b *bookFingerprinter) readEPUB(path string, fp *domain.Fingerprint) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open epub: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open opf: %w", err)
			}
			data, readErr := io.ReadAll(rc)
			_ = rc.Close()
			if readErr != nil {
				return fmt.Errorf("read opf: %w", readErr)
			}
			return parseOPF(data, fp)
		}
	}
	return fmt.Errorf("no .opf file in epub")
}

type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
}

type opfMetadata struct {
	Titles      []opfText       `xml:"http://purl.org/dc/elements/1.1/ title"`
	Creators    []opfText       `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Identifiers []opfIdentifier `xml:"http://purl.org/dc/elements/1.1/ identifier"`
}

type opfText struct {
	Value string `xml:",chardata"`
}

type opfIdentifier struct {
	Scheme    string `xml:"scheme,attr"`
	OPFScheme string `xml:"http://www.idpf.org/2007/opf scheme,attr"`
	Value     string `xml:",chardata"`
}

func (id opfIdentifier) isISBN() bool {
	return strings.EqualFold(id.Scheme, "ISBN") || strings.EqualFold(id.OPFScheme, "ISBN")
}

func parseOPF(data []byte, fp *domain.Fingerprint) error {
	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("parse opf: %w", err)
	}
	if len(pkg.Metadata.Titles) > 0 {
		fp.EmbeddedTags["title"] = strings.TrimSpace(pkg.Metadata.Titles[0].Value)
	}
	if len(pkg.Metadata.Creators) > 0 {
		fp.EmbeddedTags["creator"] = strings.TrimSpace(pkg.Metadata.Creators[0].Value)
	}
	for _, id := range pkg.Metadata.Identifiers {
		if id.isISBN() {
			fp.ISBN = strings.TrimSpace(id.Value)
			break
		}
	}
	return nil
}

func (b *bookFingerprinter) readPDF(path string, fp *domain.Fingerprint) error {
	f, err := os.Open(path) //nolint:gosec // path comes from the internal scanner, not from user HTTP input
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := pdfapi.PDFInfo(f, path, nil, false, nil)
	if err != nil {
		return fmt.Errorf("pdf info: %w", err)
	}
	if info.Title != "" {
		fp.EmbeddedTags["title"] = info.Title
	}
	if info.Author != "" {
		fp.EmbeddedTags["creator"] = info.Author
	}
	return nil
}
