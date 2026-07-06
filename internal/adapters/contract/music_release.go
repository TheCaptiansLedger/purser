package contract

import (
	"context"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"testing"
	"time"
)

func runMusicReleaseContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()
	if s.MusicReleases == nil {
		t.Skip("not implemented yet")
	}

	t.Run("SaveAndGet", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR SaveAndGet Artist", "MR SaveAndGet Album")
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "Test Album (Original Press)",
			Country:        "US",
			Date:           time.Date(1981, 1, 1, 0, 0, 0, 0, time.UTC),
			Label:          "Test Label",
			CatalogNumber:  "CAT-001",
			Barcode:        "012345678901",
			Format:         "CD",
			MediumCount:    1,
			TrackCount:     10,
			IsDefault:      true,
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if r.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.MusicReleases.Get(ctx, r.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Title != r.Title {
			t.Errorf("Title = %q, want %q", got.Title, r.Title)
		}
		if got.Country != r.Country {
			t.Errorf("Country = %q, want %q", got.Country, r.Country)
		}
		if got.Label != r.Label {
			t.Errorf("Label = %q, want %q", got.Label, r.Label)
		}
		if got.CatalogNumber != r.CatalogNumber {
			t.Errorf("CatalogNumber = %q, want %q", got.CatalogNumber, r.CatalogNumber)
		}
		if got.Barcode != r.Barcode {
			t.Errorf("Barcode = %q, want %q", got.Barcode, r.Barcode)
		}
		if got.Format != r.Format {
			t.Errorf("Format = %q, want %q", got.Format, r.Format)
		}
		if got.MediumCount != r.MediumCount {
			t.Errorf("MediumCount = %d, want %d", got.MediumCount, r.MediumCount)
		}
		if got.TrackCount != r.TrackCount {
			t.Errorf("TrackCount = %d, want %d", got.TrackCount, r.TrackCount)
		}
		if got.IsDefault != r.IsDefault {
			t.Errorf("IsDefault = %v, want %v", got.IsDefault, r.IsDefault)
		}
		if got.Status != r.Status {
			t.Errorf("Status = %q, want %q", got.Status, r.Status)
		}
		if got.GroupID != r.GroupID {
			t.Errorf("GroupID = %q, want %q", got.GroupID, r.GroupID)
		}
		if got.LibraryEntryID != r.LibraryEntryID {
			t.Errorf("LibraryEntryID = %q, want %q", got.LibraryEntryID, r.LibraryEntryID)
		}
		if !got.Date.Equal(r.Date) {
			t.Errorf("Date = %v, want %v", got.Date, r.Date)
		}
	})

	t.Run("GetByMBID", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR GetByMBID Artist", "MR GetByMBID Album")
		const mbid = "contract-mbz-release-getbymid-001"
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "GetByMBID Release",
			Status:         domain.ReleaseStatusStub,
			ExternalIDs:    []domain.ExternalID{{Source: domain.SourceMusicBrainz, Value: mbid}},
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MusicReleases.GetByMBID(ctx, mbid)
		if err != nil {
			t.Fatalf("GetByMBID: %v", err)
		}
		if got.ID != r.ID {
			t.Errorf("ID = %q, want %q", got.ID, r.ID)
		}
	})

	t.Run("GetByBarcode", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR GetByBarcode Artist", "MR GetByBarcode Album")
		const barcode = "9876543210987"
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "GetByBarcode Release",
			Barcode:        barcode,
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MusicReleases.GetByBarcode(ctx, barcode)
		if err != nil {
			t.Fatalf("GetByBarcode: %v", err)
		}
		if got.ID != r.ID {
			t.Errorf("ID = %q, want %q", got.ID, r.ID)
		}
	})

	t.Run("GetByMBIDNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.MusicReleases.GetByMBID(ctx, "no-such-mbid-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("GetByBarcodeNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.MusicReleases.GetByBarcode(ctx, "000000000000")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("ListByGroup", func(t *testing.T) {
		ctx := context.Background()
		entryA, groupA := newTestMusicGroup(ctx, t, s, "MR ListByGroup Artist A", "MR ListByGroup Album A")
		entryB, groupB := newTestMusicGroup(ctx, t, s, "MR ListByGroup Artist B", "MR ListByGroup Album B")
		for i := 0; i < 2; i++ {
			r := &domain.MusicRelease{
				GroupID:        groupA.ID,
				LibraryEntryID: entryA.ID,
				Title:          fmt.Sprintf("Album A Release %d", i+1),
				Status:         domain.ReleaseStatusStub,
			}
			if err := s.MusicReleases.Save(ctx, r); err != nil {
				t.Fatalf("Save A%d: %v", i+1, err)
			}
		}
		decoy := &domain.MusicRelease{
			GroupID:        groupB.ID,
			LibraryEntryID: entryB.ID,
			Title:          "Album B Release 1",
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, decoy); err != nil {
			t.Fatalf("Save B decoy: %v", err)
		}
		results, err := s.MusicReleases.ListByGroup(ctx, groupA.ID)
		if err != nil {
			t.Fatalf("ListByGroup: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("ListByGroup count = %d, want 2", len(results))
		}
		for _, got := range results {
			if got.GroupID != groupA.ID {
				t.Errorf("GroupID = %q, want %q (filter leaked another group)", got.GroupID, groupA.ID)
			}
		}
	})

	t.Run("ListByEntry", func(t *testing.T) {
		ctx := context.Background()
		entryA, groupA := newTestMusicGroup(ctx, t, s, "MR ListByEntry Artist A", "MR ListByEntry Album A")
		entryB, groupB := newTestMusicGroup(ctx, t, s, "MR ListByEntry Artist B", "MR ListByEntry Album B")
		for i := 0; i < 2; i++ {
			r := &domain.MusicRelease{
				GroupID:        groupA.ID,
				LibraryEntryID: entryA.ID,
				Title:          fmt.Sprintf("Entry A Release %d", i+1),
				Status:         domain.ReleaseStatusStub,
			}
			if err := s.MusicReleases.Save(ctx, r); err != nil {
				t.Fatalf("Save A%d: %v", i+1, err)
			}
		}
		decoy := &domain.MusicRelease{
			GroupID:        groupB.ID,
			LibraryEntryID: entryB.ID,
			Title:          "Entry B Release 1",
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, decoy); err != nil {
			t.Fatalf("Save B decoy: %v", err)
		}
		results, err := s.MusicReleases.ListByEntry(ctx, entryA.ID)
		if err != nil {
			t.Fatalf("ListByEntry: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("ListByEntry count = %d, want 2", len(results))
		}
		for _, got := range results {
			if got.LibraryEntryID != entryA.ID {
				t.Errorf("LibraryEntryID = %q, want %q (filter leaked another entry)", got.LibraryEntryID, entryA.ID)
			}
		}
	})

	t.Run("ListTracksByRelease", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR ListTracks Artist", "MR ListTracks Album")
		release := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "ListTracks Release",
			TrackCount:     2,
			Status:         domain.ReleaseStatusImported,
		}
		if err := s.MusicReleases.Save(ctx, release); err != nil {
			t.Fatalf("Save release: %v", err)
		}
		for i := 0; i < 2; i++ {
			item := &domain.Item{
				ContentType:    domain.ContentTypeMusic,
				LibraryEntryID: entry.ID,
				GroupID:        group.ID,
				Title:          fmt.Sprintf("Track %d", i+1),
				Status:         domain.StatusImported,
				Sequence:       fmt.Sprintf("%d", i+1),
				Metadata:       map[string]any{"release_id": release.ID},
			}
			if err := s.Items.Save(ctx, item); err != nil {
				t.Fatalf("Save item %d: %v", i+1, err)
			}
		}
		decoy := &domain.Item{
			ContentType:    domain.ContentTypeMusic,
			LibraryEntryID: entry.ID,
			GroupID:        group.ID,
			Title:          "Decoy Track",
			Status:         domain.StatusWanted,
		}
		if err := s.Items.Save(ctx, decoy); err != nil {
			t.Fatalf("Save decoy: %v", err)
		}
		tracks, err := s.MusicReleases.ListTracksByRelease(ctx, release.ID)
		if err != nil {
			t.Fatalf("ListTracksByRelease: %v", err)
		}
		if len(tracks) != 2 {
			t.Errorf("ListTracksByRelease count = %d, want 2", len(tracks))
		}
	})

	t.Run("StatusRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR StatusRT Artist", "MR StatusRT Album")
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "StatusRT Release",
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MusicReleases.Get(ctx, r.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Status != domain.ReleaseStatusStub {
			t.Errorf("Status = %q, want stub", got.Status)
		}
		r.Status = domain.ReleaseStatusImported
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save imported: %v", err)
		}
		got, err = s.MusicReleases.Get(ctx, r.ID)
		if err != nil {
			t.Fatalf("Get after update: %v", err)
		}
		if got.Status != domain.ReleaseStatusImported {
			t.Errorf("Status = %q, want imported", got.Status)
		}
	})

	t.Run("IsDefaultRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR IsDefault Artist", "MR IsDefault Album")
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "IsDefault Release",
			IsDefault:      true,
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MusicReleases.Get(ctx, r.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !got.IsDefault {
			t.Error("IsDefault = false, want true")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR Delete Artist", "MR Delete Album")
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "Delete Release",
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := s.MusicReleases.Delete(ctx, r.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := s.MusicReleases.Get(ctx, r.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("StaleIndexCleanedOnResave", func(t *testing.T) {
		ctx := context.Background()
		entry, group := newTestMusicGroup(ctx, t, s, "MR StaleIndex Artist", "MR StaleIndex Album")
		const barcodeA = "stale-barcode-A-001"
		const barcodeB = "stale-barcode-B-001"
		r := &domain.MusicRelease{
			GroupID:        group.ID,
			LibraryEntryID: entry.ID,
			Title:          "StaleIndex Release",
			Barcode:        barcodeA,
			Status:         domain.ReleaseStatusStub,
		}
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save initial: %v", err)
		}
		r.Barcode = barcodeB
		if err := s.MusicReleases.Save(ctx, r); err != nil {
			t.Fatalf("Save updated: %v", err)
		}
		_, err := s.MusicReleases.GetByBarcode(ctx, barcodeA)
		if !errs.IsNotFound(err) {
			t.Errorf("old barcode still resolvable after resave: want ErrNotFound, got %v", err)
		}
		got, err := s.MusicReleases.GetByBarcode(ctx, barcodeB)
		if err != nil {
			t.Fatalf("new barcode not found after resave: %v", err)
		}
		if got.ID != r.ID {
			t.Errorf("ID = %q, want %q", got.ID, r.ID)
		}
	})
}
