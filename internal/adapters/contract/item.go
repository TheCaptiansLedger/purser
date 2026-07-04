package contract

import (
	"context"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func runItemContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveAndGet", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeAdult, domain.KindStudio, "Item SaveAndGet Studio")
		item := &domain.Item{
			ContentType:    domain.ContentTypeAdult,
			LibraryEntryID: entry.ID,
			Title:          "Contract Scene",
			Overview:       "overview",
			Monitored:      true,
			Status:         domain.StatusWanted,
		}
		if err := s.Items.Save(ctx, item); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if item.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.Items.Get(ctx, item.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Title != item.Title {
			t.Errorf("Title = %q, want %q", got.Title, item.Title)
		}
		if got.Overview != item.Overview {
			t.Errorf("Overview = %q, want %q", got.Overview, item.Overview)
		}
		if got.ContentType != item.ContentType {
			t.Errorf("ContentType = %q, want %q", got.ContentType, item.ContentType)
		}
		if got.Status != item.Status {
			t.Errorf("Status = %q, want %q", got.Status, item.Status)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.Items.Get(ctx, "no-such-item-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	// Regression: itemFromRecord in the Badger adapter never loaded the kMFI
	// index so item.MediaFile was always nil on Get even when a media file
	// existed. Both Get and List must populate MediaFile.
	t.Run("GetPopulatesMediaFileWhenPresent", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "MF Regression Get")
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/regression/get-populates-mf.mp4",
			Size:   12345,
			OSHash: "regression-get-001",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save media file: %v", err)
		}
		got, err := s.Items.Get(ctx, item.ID)
		if err != nil {
			t.Fatalf("Get item: %v", err)
		}
		if got.MediaFile == nil {
			t.Fatal("item.MediaFile is nil after media file was saved — backend does not load MediaFile on Get")
		}
		if got.MediaFile.Path != mf.Path {
			t.Errorf("MediaFile.Path = %q, want %q", got.MediaFile.Path, mf.Path)
		}
		if got.MediaFile.Size != mf.Size {
			t.Errorf("MediaFile.Size = %d, want %d", got.MediaFile.Size, mf.Size)
		}
	})

	t.Run("ListPopulatesMediaFileWhenPresent", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeAdult, domain.KindStudio, "MF Regression List Studio")
		item := &domain.Item{
			ContentType:    domain.ContentTypeAdult,
			LibraryEntryID: entry.ID,
			Title:          "MF Regression List Scene",
			Status:         domain.StatusImported,
			Monitored:      true,
		}
		if err := s.Items.Save(ctx, item); err != nil {
			t.Fatalf("Save item: %v", err)
		}
		mf := &domain.MediaFile{
			ItemID: item.ID,
			Path:   "/regression/list-populates-mf.mp4",
			Size:   67890,
			OSHash: "regression-list-002",
		}
		if err := s.MediaFiles.Save(ctx, mf); err != nil {
			t.Fatalf("Save media file: %v", err)
		}
		items, _, err := s.Items.List(ctx, ports.ItemFilter{
			LibraryEntryID: entry.ID,
			Limit:          10,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item in List, got %d", len(items))
		}
		if items[0].MediaFile == nil {
			t.Fatal("item.MediaFile is nil in List results — backend does not load MediaFile on List")
		}
		if items[0].MediaFile.Path != mf.Path {
			t.Errorf("MediaFile.Path = %q, want %q", items[0].MediaFile.Path, mf.Path)
		}
	})

	t.Run("GetReturnsNilMediaFileWhenAbsent", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "No MF Item")
		got, err := s.Items.Get(ctx, item.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.MediaFile != nil {
			t.Errorf("MediaFile should be nil when no media file exists, got %+v", got.MediaFile)
		}
	})

	t.Run("ListByContentType", func(t *testing.T) {
		ctx := context.Background()
		adultEntry := newTestEntry(ctx, t, s, domain.ContentTypeAdult, domain.KindStudio, "CT Filter Adult Studio")
		musicEntry := newTestEntry(ctx, t, s, domain.ContentTypeMusic, domain.KindArtist, "CT Filter Music Artist")
		adult := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: adultEntry.ID, Title: "CT Adult Scene", Status: domain.StatusWanted, Monitored: true}
		music := &domain.Item{ContentType: domain.ContentTypeMusic, LibraryEntryID: musicEntry.ID, Title: "CT Music Track", Status: domain.StatusWanted, Monitored: true}
		for _, item := range []*domain.Item{adult, music} {
			if err := s.Items.Save(ctx, item); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, _, err := s.Items.List(ctx, ports.ItemFilter{
			ContentTypes: []domain.ContentType{domain.ContentTypeAdult},
			Limit:        100,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, r := range results {
			if r.ContentType != domain.ContentTypeAdult {
				t.Errorf("ContentType filter leaked item with type %q", r.ContentType)
			}
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeAdult, domain.KindStudio, "Status Filter Studio")
		wanted := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Wanted Scene", Status: domain.StatusWanted, Monitored: true}
		imported := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: entry.ID, Title: "Imported Scene", Status: domain.StatusImported, Monitored: true}
		for _, item := range []*domain.Item{wanted, imported} {
			if err := s.Items.Save(ctx, item); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, _, err := s.Items.List(ctx, ports.ItemFilter{
			LibraryEntryID: entry.ID,
			Status:         domain.StatusWanted,
			Limit:          100,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, r := range results {
			if r.Status != domain.StatusWanted {
				t.Errorf("status filter leaked item with status %q", r.Status)
			}
		}
		if len(results) != 1 {
			t.Errorf("status filter returned %d items, want 1", len(results))
		}
	})

	t.Run("ListByPersonID", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeAdult, domain.KindStudio, "PersonID Filter Studio")
		p1 := &domain.Person{Name: "PersonID Filter P1", MonitorMode: domain.MonitorAll}
		p2 := &domain.Person{Name: "PersonID Filter P2", MonitorMode: domain.MonitorAll}
		for _, p := range []*domain.Person{p1, p2} {
			if err := s.People.Save(ctx, p); err != nil {
				t.Fatalf("Save person: %v", err)
			}
		}
		i1 := &domain.Item{
			ContentType:    domain.ContentTypeAdult,
			LibraryEntryID: entry.ID,
			Title:          "Scene P1 Only",
			Status:         domain.StatusWanted,
			People:         []domain.ItemPerson{{PersonID: p1.ID, Role: domain.RolePerformer}},
		}
		i2 := &domain.Item{
			ContentType:    domain.ContentTypeAdult,
			LibraryEntryID: entry.ID,
			Title:          "Scene P1 and P2",
			Status:         domain.StatusWanted,
			People: []domain.ItemPerson{
				{PersonID: p1.ID, Role: domain.RolePerformer},
				{PersonID: p2.ID, Role: domain.RolePerformer},
			},
		}
		for _, item := range []*domain.Item{i1, i2} {
			if err := s.Items.Save(ctx, item); err != nil {
				t.Fatalf("Save item: %v", err)
			}
		}
		results, _, err := s.Items.List(ctx, ports.ItemFilter{PersonID: p1.ID, Limit: 100})
		if err != nil {
			t.Fatalf("List by p1: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("p1 filter: got %d items, want 2", len(results))
		}
		results, _, err = s.Items.List(ctx, ports.ItemFilter{PersonID: p2.ID, Limit: 100})
		if err != nil {
			t.Fatalf("List by p2: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("p2 filter: got %d items, want 1", len(results))
		}
	})

	t.Run("ListByGroupID", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeTV, domain.KindSeries, "GroupID Filter Series")
		g1 := &domain.Group{LibraryEntryID: entry.ID, Title: "GroupID S1", Monitored: true, MonitorMode: domain.MonitorAll}
		g2 := &domain.Group{LibraryEntryID: entry.ID, Title: "GroupID S2", Monitored: true, MonitorMode: domain.MonitorAll}
		for _, g := range []*domain.Group{g1, g2} {
			if err := s.Groups.Save(ctx, g); err != nil {
				t.Fatalf("Save group: %v", err)
			}
		}
		for i := 0; i < 3; i++ {
			ep := &domain.Item{ContentType: domain.ContentTypeTV, LibraryEntryID: entry.ID, GroupID: g1.ID, Title: fmt.Sprintf("S1E%d", i+1), Status: domain.StatusWanted}
			if err := s.Items.Save(ctx, ep); err != nil {
				t.Fatalf("Save item: %v", err)
			}
		}
		ep := &domain.Item{ContentType: domain.ContentTypeTV, LibraryEntryID: entry.ID, GroupID: g2.ID, Title: "S2E1", Status: domain.StatusWanted}
		if err := s.Items.Save(ctx, ep); err != nil {
			t.Fatalf("Save item: %v", err)
		}
		results, _, err := s.Items.List(ctx, ports.ItemFilter{GroupID: g1.ID, Limit: 100})
		if err != nil {
			t.Fatalf("List by group: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("group filter: got %d items, want 3", len(results))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		item := newTestItem(ctx, t, s, "Delete Me Item")
		if err := s.Items.Delete(ctx, item.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := s.Items.Get(ctx, item.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteByGroup", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeTV, domain.KindSeries, "DeleteByGroup Series")
		group := &domain.Group{LibraryEntryID: entry.ID, Title: "DeleteByGroup Season", Monitored: true, MonitorMode: domain.MonitorAll}
		if err := s.Groups.Save(ctx, group); err != nil {
			t.Fatalf("Save group: %v", err)
		}
		for i := 0; i < 2; i++ {
			ep := &domain.Item{ContentType: domain.ContentTypeTV, LibraryEntryID: entry.ID, GroupID: group.ID, Title: fmt.Sprintf("DBG E%d", i+1), Status: domain.StatusWanted}
			if err := s.Items.Save(ctx, ep); err != nil {
				t.Fatalf("Save item: %v", err)
			}
		}
		if err := s.Items.DeleteByGroup(ctx, group.ID); err != nil {
			t.Fatalf("DeleteByGroup: %v", err)
		}
		remaining, _, err := s.Items.List(ctx, ports.ItemFilter{GroupID: group.ID, Limit: 100})
		if err != nil {
			t.Fatalf("List after delete: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("after DeleteByGroup: %d items remain, want 0", len(remaining))
		}
	})

	t.Run("ExternalIDsRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		item := &domain.Item{
			ContentType: domain.ContentTypeMusic,
			Title:       "ExternalID Track",
			Status:      domain.StatusImported,
			ExternalIDs: []domain.ExternalID{
				{Source: domain.SourceMusicBrainz, Value: "contract-mbid-001"},
			},
		}
		entry := newTestEntry(ctx, t, s, domain.ContentTypeMusic, domain.KindArtist, "ExternalID Artist")
		item.LibraryEntryID = entry.ID
		if err := s.Items.Save(ctx, item); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Items.Get(ctx, item.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got.ExternalIDs) != 1 {
			t.Fatalf("ExternalIDs len = %d, want 1", len(got.ExternalIDs))
		}
		if got.ExternalIDs[0].Source != domain.SourceMusicBrainz {
			t.Errorf("ExternalID.Source = %q, want %q", got.ExternalIDs[0].Source, domain.SourceMusicBrainz)
		}
		if got.ExternalIDs[0].Value != "contract-mbid-001" {
			t.Errorf("ExternalID.Value = %q, want contract-mbid-001", got.ExternalIDs[0].Value)
		}
	})
}
