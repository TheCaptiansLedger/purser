package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func runLibraryEntryContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveAndGet", func(t *testing.T) {
		ctx := context.Background()
		entry := &domain.LibraryEntry{
			ContentType: domain.ContentTypeAdult,
			Kind:        domain.KindStudio,
			Name:        "Contract Studio",
			Overview:    "overview text",
			MonitorMode: domain.MonitorAll,
			Status:      domain.EntryStatusActive,
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if entry.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.LibraryEntries.Get(ctx, entry.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Name != entry.Name {
			t.Errorf("Name = %q, want %q", got.Name, entry.Name)
		}
		if got.Overview != entry.Overview {
			t.Errorf("Overview = %q, want %q", got.Overview, entry.Overview)
		}
		if got.ContentType != entry.ContentType {
			t.Errorf("ContentType = %q, want %q", got.ContentType, entry.ContentType)
		}
		if got.Kind != entry.Kind {
			t.Errorf("Kind = %q, want %q", got.Kind, entry.Kind)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.LibraryEntries.Get(ctx, "no-such-entry-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		ctx := context.Background()
		entry := &domain.LibraryEntry{
			ContentType: domain.ContentTypeTV,
			Kind:        domain.KindSeries,
			Name:        "Before Update",
			MonitorMode: domain.MonitorAll,
			Status:      domain.EntryStatusActive,
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save: %v", err)
		}
		entry.Name = "After Update"
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("update Save: %v", err)
		}
		got, _ := s.LibraryEntries.Get(ctx, entry.ID)
		if got.Name != "After Update" {
			t.Errorf("after update Name = %q, want After Update", got.Name)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		entry := &domain.LibraryEntry{
			ContentType: domain.ContentTypeMusic,
			Kind:        domain.KindArtist,
			Name:        "Delete Me Entry",
			MonitorMode: domain.MonitorNone,
			Status:      domain.EntryStatusActive,
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := s.LibraryEntries.Delete(ctx, entry.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := s.LibraryEntries.Get(ctx, entry.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("ListByContentType", func(t *testing.T) {
		ctx := context.Background()
		adult := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "CTFilter Adult", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
		music := &domain.LibraryEntry{ContentType: domain.ContentTypeMusic, Kind: domain.KindArtist, Name: "CTFilter Music", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
		for _, e := range []*domain.LibraryEntry{adult, music} {
			if err := s.LibraryEntries.Save(ctx, e); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, _, err := s.LibraryEntries.List(ctx, ports.LibraryFilter{ContentType: domain.ContentTypeAdult, Limit: 100})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, r := range results {
			if r.ContentType != domain.ContentTypeAdult {
				t.Errorf("ContentType filter leaked entry with type %q", r.ContentType)
			}
		}
	})

	t.Run("ListBySearch", func(t *testing.T) {
		ctx := context.Background()
		target := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "UniqueSearchStudio9977", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
		other := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "OtherStudio9977", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive}
		for _, e := range []*domain.LibraryEntry{target, other} {
			if err := s.LibraryEntries.Save(ctx, e); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, _, err := s.LibraryEntries.List(ctx, ports.LibraryFilter{Search: "UniqueSearchStudio9977", Limit: 10})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(results) != 1 || results[0].ID != target.ID {
			t.Errorf("search returned %d results, want 1 (the target)", len(results))
		}
	})

	t.Run("LockedFieldsRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		entry := &domain.LibraryEntry{
			ContentType:  domain.ContentTypeTV,
			Kind:         domain.KindSeries,
			Name:         "Locked Fields Entry",
			MonitorMode:  domain.MonitorAll,
			Status:       domain.EntryStatusActive,
			LockedFields: []string{"name", "overview", "status"},
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.LibraryEntries.Get(ctx, entry.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got.LockedFields) != 3 {
			t.Errorf("LockedFields len = %d, want 3: %v", len(got.LockedFields), got.LockedFields)
		}
	})

	t.Run("PeopleManagement", func(t *testing.T) {
		ctx := context.Background()
		entry := &domain.LibraryEntry{
			ContentType: domain.ContentTypeAdult,
			Kind:        domain.KindStudio,
			Name:        "People Entry",
			MonitorMode: domain.MonitorAll,
			Status:      domain.EntryStatusActive,
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save entry: %v", err)
		}
		person := &domain.Person{Name: "Entry Person", MonitorMode: domain.MonitorAll}
		if err := s.People.Save(ctx, person); err != nil {
			t.Fatalf("Save person: %v", err)
		}
		ep := domain.EntryPerson{PersonID: person.ID, Role: "performer"}
		if err := s.LibraryEntries.SavePerson(ctx, entry.ID, ep); err != nil {
			t.Fatalf("SavePerson: %v", err)
		}
		people, err := s.LibraryEntries.GetPeople(ctx, entry.ID)
		if err != nil {
			t.Fatalf("GetPeople: %v", err)
		}
		if len(people) != 1 || people[0].PersonID != person.ID {
			t.Errorf("GetPeople = %v, want 1 person with ID %q", people, person.ID)
		}
		if err := s.LibraryEntries.RemovePerson(ctx, entry.ID, person.ID, "performer"); err != nil {
			t.Fatalf("RemovePerson: %v", err)
		}
		people, err = s.LibraryEntries.GetPeople(ctx, entry.ID)
		if err != nil {
			t.Fatalf("GetPeople after remove: %v", err)
		}
		if len(people) != 0 {
			t.Errorf("after RemovePerson GetPeople = %d, want 0", len(people))
		}
	})
}
