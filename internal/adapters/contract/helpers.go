package contract

import (
	"context"
	"purser/internal/domain"
	"testing"
)

// newTestEntry creates and saves a minimal LibraryEntry for use as a parent
// record in other contract tests. The SQL adapter enforces FK constraints so
// items and groups always need a real entry.
func newTestEntry(ctx context.Context, t *testing.T, s BackendSuite, ct domain.ContentType, kind domain.Kind, name string) *domain.LibraryEntry {
	t.Helper()
	entry := &domain.LibraryEntry{
		ContentType: ct,
		Kind:        kind,
		Name:        name,
		MonitorMode: domain.MonitorAll,
		Status:      domain.EntryStatusActive,
	}
	if err := s.LibraryEntries.Save(ctx, entry); err != nil {
		t.Fatalf("newTestEntry Save %q: %v", name, err)
	}
	return entry
}

// newTestMusicGroup creates a music LibraryEntry (artist) and a Group (album) under it.
func newTestMusicGroup(ctx context.Context, t *testing.T, s BackendSuite, artistName, albumTitle string) (*domain.LibraryEntry, *domain.Group) {
	t.Helper()
	entry := newTestEntry(ctx, t, s, domain.ContentTypeMusic, domain.KindArtist, artistName)
	group := &domain.Group{
		LibraryEntryID: entry.ID,
		Title:          albumTitle,
		Monitored:      true,
		MonitorMode:    domain.MonitorAll,
	}
	if err := s.Groups.Save(ctx, group); err != nil {
		t.Fatalf("newTestMusicGroup Save %q: %v", albumTitle, err)
	}
	return entry, group
}

// newTestItem creates and saves a minimal Item under a new LibraryEntry.
func newTestItem(ctx context.Context, t *testing.T, s BackendSuite, title string) *domain.Item {
	t.Helper()
	entry := newTestEntry(ctx, t, s, domain.ContentTypeAdult, domain.KindStudio, title+" (entry)")
	item := &domain.Item{
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: entry.ID,
		Title:          title,
		Status:         domain.StatusWanted,
		Monitored:      true,
	}
	if err := s.Items.Save(ctx, item); err != nil {
		t.Fatalf("newTestItem Save %q: %v", title, err)
	}
	return item
}
