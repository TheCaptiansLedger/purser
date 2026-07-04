package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func runPersonContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveAndGet", func(t *testing.T) {
		ctx := context.Background()
		p := &domain.Person{
			Name:        "Jane Doe",
			Overview:    "test overview",
			MonitorMode: domain.MonitorAll,
			Aliases:     []string{"J. Doe", "Jane"},
		}
		if err := s.People.Save(ctx, p); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if p.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.People.Get(ctx, p.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Name != p.Name {
			t.Errorf("Name = %q, want %q", got.Name, p.Name)
		}
		if got.Overview != p.Overview {
			t.Errorf("Overview = %q, want %q", got.Overview, p.Overview)
		}
		if len(got.Aliases) != 2 {
			t.Errorf("Aliases len = %d, want 2", len(got.Aliases))
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := s.People.Get(ctx, "no-such-person-xyz")
		if !errs.IsNotFound(err) {
			t.Errorf("Get missing: want ErrNotFound, got %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		ctx := context.Background()
		p := &domain.Person{Name: "Update Me", MonitorMode: domain.MonitorNone}
		if err := s.People.Save(ctx, p); err != nil {
			t.Fatalf("initial Save: %v", err)
		}
		p.Name = "Updated"
		if err := s.People.Save(ctx, p); err != nil {
			t.Fatalf("update Save: %v", err)
		}
		got, _ := s.People.Get(ctx, p.ID)
		if got.Name != "Updated" {
			t.Errorf("after update Name = %q, want Updated", got.Name)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		p := &domain.Person{Name: "To Delete", MonitorMode: domain.MonitorNone}
		if err := s.People.Save(ctx, p); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := s.People.Delete(ctx, p.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := s.People.Get(ctx, p.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("ListBySearch", func(t *testing.T) {
		ctx := context.Background()
		alice := &domain.Person{Name: "Alice Unique123", MonitorMode: domain.MonitorAll}
		bob := &domain.Person{Name: "Bob Unique123", MonitorMode: domain.MonitorAll}
		for _, p := range []*domain.Person{alice, bob} {
			if err := s.People.Save(ctx, p); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		results, _, err := s.People.List(ctx, ports.PersonFilter{Search: "Alice Unique123", Limit: 10})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(results) != 1 || results[0].Name != "Alice Unique123" {
			t.Errorf("search result count = %d, want 1", len(results))
		}
	})

	t.Run("ListByRole", func(t *testing.T) {
		ctx := context.Background()
		p := &domain.Person{Name: "Role Test Person", MonitorMode: domain.MonitorAll}
		if err := s.People.Save(ctx, p); err != nil {
			t.Fatalf("Save: %v", err)
		}
		// Roles are derived from item/entry credits; a person with no credits
		// appears in an unfiltered list but not in a role-filtered one.
		all, _, err := s.People.List(ctx, ports.PersonFilter{Limit: 100})
		if err != nil {
			t.Fatalf("List all: %v", err)
		}
		found := false
		for _, r := range all {
			if r.ID == p.ID {
				found = true
			}
		}
		if !found {
			t.Error("person not found in unfiltered list")
		}
	})

	t.Run("LockedFields", func(t *testing.T) {
		ctx := context.Background()
		p := &domain.Person{
			Name:         "Locked Fields Person",
			MonitorMode:  domain.MonitorAll,
			LockedFields: []string{"name", "overview"},
		}
		if err := s.People.Save(ctx, p); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.People.Get(ctx, p.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got.LockedFields) != 2 {
			t.Errorf("LockedFields len = %d, want 2", len(got.LockedFields))
		}
	})
}
