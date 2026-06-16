package domain

import "testing"

func TestLibraryEntry_IsRoot(t *testing.T) {
	e := &LibraryEntry{}
	if !e.IsRoot() {
		t.Error("IsRoot() = false for empty ParentID, want true")
	}
	e.ParentID = "parent-123"
	if e.IsRoot() {
		t.Error("IsRoot() = true for non-empty ParentID, want false")
	}
}

func TestLibraryEntry_DisplayName(t *testing.T) {
	e := &LibraryEntry{Name: "Evil Angel"}
	if got := e.DisplayName(); got != "Evil Angel" {
		t.Errorf("DisplayName() = %q, want %q", got, "Evil Angel")
	}

	e.SortName = "Angel, Evil"
	if got := e.DisplayName(); got != "Angel, Evil" {
		t.Errorf("DisplayName() with SortName = %q, want %q", got, "Angel, Evil")
	}
}
