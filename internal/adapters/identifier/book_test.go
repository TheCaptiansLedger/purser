package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"testing"
)

func TestBookIdentifier_ContentTypes(t *testing.T) {
	id := identifier.NewBookIdentifier(&stubExternalIDRepo{}, &stubItemRepo{}, nil)
	cts := id.ContentTypes()
	if len(cts) != 1 || cts[0] != domain.ContentTypeBook {
		t.Errorf("ContentTypes() = %v, want [book]", cts)
	}
}

func TestBookIdentifier_UnsupportedType(t *testing.T) {
	id := identifier.NewBookIdentifier(&stubExternalIDRepo{}, &stubItemRepo{}, nil)
	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/fake/movie.mkv",
		ContentType: domain.ContentTypeMovie,
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidates != nil {
		t.Errorf("expected nil for unsupported content type, got %v", candidates)
	}
}

func TestBookIdentifier_ISBNLocalMatch(t *testing.T) {
	const isbn = "9780743273565"
	const itemID = "item-book-001"

	item := &domain.Item{ID: itemID, Title: "The Great Gatsby", ContentType: domain.ContentTypeBook}
	extIDs := &stubExternalIDRepo{
		entries: map[string]string{
			"item:openlibrary:" + isbn: itemID,
		},
	}
	itemRepo := &stubItemRepo{byID: map[string]*domain.Item{itemID: item}}

	id := identifier.NewBookIdentifier(extIDs, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/books/great-gatsby.epub",
		ContentType: domain.ContentTypeBook,
		Fingerprint: &domain.Fingerprint{
			ISBN:         isbn,
			EmbeddedTags: map[string]string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Confidence != 0.95 {
		t.Errorf("confidence = %.2f, want 0.95", candidates[0].Confidence)
	}
	if candidates[0].Source != "isbn_local" {
		t.Errorf("source = %q, want isbn_local", candidates[0].Source)
	}
}

func TestBookIdentifier_TitleAuthorMatch(t *testing.T) {
	item := &domain.Item{ID: "item-book-002", Title: "Dune", ContentType: domain.ContentTypeBook}
	extIDs := &stubExternalIDRepo{entries: map[string]string{}}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewBookIdentifier(extIDs, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/books/dune.epub",
		ContentType: domain.ContentTypeBook,
		Fingerprint: &domain.Fingerprint{
			EmbeddedTags: map[string]string{
				"title":   "Dune",
				"creator": "Frank Herbert",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected candidates from title+author, got none")
	}
	if candidates[0].Confidence != 0.65 {
		t.Errorf("title+author confidence = %.2f, want 0.65", candidates[0].Confidence)
	}
	if candidates[0].Source != "title" {
		t.Errorf("source = %q, want title", candidates[0].Source)
	}
}

func TestBookIdentifier_TitleOnlyMatch(t *testing.T) {
	item := &domain.Item{ID: "item-book-003", Title: "Foundation", ContentType: domain.ContentTypeBook}
	extIDs := &stubExternalIDRepo{entries: map[string]string{}}
	itemRepo := &stubItemRepo{bySearch: []*domain.Item{item}}

	id := identifier.NewBookIdentifier(extIDs, itemRepo, nil)

	candidates, err := id.Identify(context.Background(), domain.ScannedFile{
		Path:        "/books/foundation.epub",
		ContentType: domain.ContentTypeBook,
		Fingerprint: &domain.Fingerprint{
			EmbeddedTags: map[string]string{
				"title": "Foundation",
				// no creator
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected title-only candidates, got none")
	}
	if candidates[0].Confidence != 0.40 {
		t.Errorf("title-only confidence = %.2f, want 0.40", candidates[0].Confidence)
	}
}
