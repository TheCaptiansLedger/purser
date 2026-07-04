package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"testing"
)

func runExternalIDContract(t *testing.T, s BackendSuite) {
	t.Helper()

	t.Run("FindEntityByItemExternalID", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeMusic, domain.KindArtist, "EID Artist")
		item := &domain.Item{
			ContentType:    domain.ContentTypeMusic,
			LibraryEntryID: entry.ID,
			Title:          "EID Track",
			Status:         domain.StatusImported,
			ExternalIDs: []domain.ExternalID{
				{Source: domain.SourceMusicBrainz, Value: "eid-contract-mbid-001"},
			},
		}
		if err := s.Items.Save(ctx, item); err != nil {
			t.Fatalf("Save item: %v", err)
		}

		entityID, err := s.ExternalIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), "eid-contract-mbid-001")
		if err != nil {
			t.Fatalf("FindEntity: %v", err)
		}
		if entityID != item.ID {
			t.Errorf("FindEntity returned %q, want %q", entityID, item.ID)
		}
	})

	// Source value must be stored and queried exactly as given by the domain
	// constant — "mbz", NOT "musicbrainz". This is the bug from issue #359.
	t.Run("SourceStringIsRespected", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeMusic, domain.KindArtist, "Source String Artist")
		item := &domain.Item{
			ContentType:    domain.ContentTypeMusic,
			LibraryEntryID: entry.ID,
			Title:          "Source String Track",
			Status:         domain.StatusImported,
			ExternalIDs: []domain.ExternalID{
				{Source: domain.SourceMusicBrainz, Value: "source-str-test-002"},
			},
		}
		if err := s.Items.Save(ctx, item); err != nil {
			t.Fatalf("Save item: %v", err)
		}

		// Must find by the canonical constant value ("mbz").
		id, err := s.ExternalIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), "source-str-test-002")
		if err != nil {
			t.Fatalf("FindEntity with canonical source %q: %v", domain.SourceMusicBrainz, err)
		}
		if id != item.ID {
			t.Errorf("canonical source lookup returned %q, want %q", id, item.ID)
		}

		// Must NOT find by the legacy mismatched string.
		_, err = s.ExternalIDs.FindEntity(ctx, "item", "musicbrainz", "source-str-test-002")
		if !errs.IsNotFound(err) {
			t.Errorf("legacy source string 'musicbrainz' should return ErrNotFound, got %v (entity %q)", err, id)
		}
	})

	t.Run("FindEntityNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.ExternalIDs.FindEntity(ctx, "item", string(domain.SourceMusicBrainz), "no-such-mbid-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("FindEntityByLibraryEntryExternalID", func(t *testing.T) {
		ctx := context.Background()
		entry := &domain.LibraryEntry{
			ContentType: domain.ContentTypeMusic,
			Kind:        domain.KindArtist,
			Name:        "EID Entry Artist",
			MonitorMode: domain.MonitorAll,
			Status:      domain.EntryStatusActive,
			ExternalIDs: []domain.ExternalID{
				{Source: domain.SourceMusicBrainz, Value: "eid-entry-mbid-003"},
			},
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save entry: %v", err)
		}

		entityID, err := s.ExternalIDs.FindEntity(ctx, "library_entry", string(domain.SourceMusicBrainz), "eid-entry-mbid-003")
		if err != nil {
			t.Fatalf("FindEntity for entry: %v", err)
		}
		if entityID != entry.ID {
			t.Errorf("FindEntity returned %q, want %q", entityID, entry.ID)
		}
	})
}
