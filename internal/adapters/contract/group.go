package contract

import (
	"context"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func runGroupContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveGetListDelete", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeTV, domain.KindSeries, "Group Contract Series")

		g1 := &domain.Group{LibraryEntryID: entry.ID, Title: "Season 1", Number: 1, Monitored: true, MonitorMode: domain.MonitorAll}
		g2 := &domain.Group{LibraryEntryID: entry.ID, Title: "Season 2", Number: 2, Monitored: false, MonitorMode: domain.MonitorNone}
		for _, g := range []*domain.Group{g1, g2} {
			if err := s.Groups.Save(ctx, g); err != nil {
				t.Fatalf("Save group: %v", err)
			}
			if g.ID == "" {
				t.Fatal("Save must set ID")
			}
		}

		got, err := s.Groups.Get(ctx, g1.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Title != "Season 1" {
			t.Errorf("Title = %q, want Season 1", got.Title)
		}
		if got.Number != 1 {
			t.Errorf("Number = %d, want 1", got.Number)
		}

		all, err := s.Groups.List(ctx, ports.GroupFilter{LibraryEntryID: entry.ID})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("List count = %d, want 2", len(all))
		}

		if err := s.Groups.Delete(ctx, g1.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err = s.Groups.Get(ctx, g1.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteByLibraryEntry", func(t *testing.T) {
		ctx := context.Background()
		entry := newTestEntry(ctx, t, s, domain.ContentTypeTV, domain.KindSeries, "DeleteByEntry Series")
		for i := 0; i < 3; i++ {
			g := &domain.Group{LibraryEntryID: entry.ID, Title: fmt.Sprintf("Season %d", i+1), Monitored: true, MonitorMode: domain.MonitorAll}
			if err := s.Groups.Save(ctx, g); err != nil {
				t.Fatalf("Save group: %v", err)
			}
		}
		if err := s.Groups.DeleteByLibraryEntry(ctx, entry.ID); err != nil {
			t.Fatalf("DeleteByLibraryEntry: %v", err)
		}
		remaining, err := s.Groups.List(ctx, ports.GroupFilter{LibraryEntryID: entry.ID})
		if err != nil {
			t.Fatalf("List after delete: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("after DeleteByLibraryEntry: %d groups remain, want 0", len(remaining))
		}
	})
}
