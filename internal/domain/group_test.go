package domain

import "testing"

func validGroup() Group {
	return Group{
		ID:             "g1",
		LibraryEntryID: "le1",
		Title:          "Rumours",
		MonitorMode:    MonitorModeAll,
	}
}

func TestGroup_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Group)
		wantErr bool
	}{
		{"valid", func(_ *Group) {}, false},
		{"valid with fractional-style number", func(g *Group) { g.Number = "1.5" }, false},
		{"missing ID", func(g *Group) { g.ID = "" }, true},
		{"missing LibraryEntryID", func(g *Group) { g.LibraryEntryID = "" }, true},
		{"missing Title", func(g *Group) { g.Title = "" }, true},
		{"missing MonitorMode", func(g *Group) { g.MonitorMode = "" }, true},
		{"invalid MonitorMode", func(g *Group) { g.MonitorMode = "sometimes" }, true},
		{"latest monitor mode is valid", func(g *Group) { g.MonitorMode = MonitorModeLatest }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := validGroup()
			tt.mutate(&g)
			err := g.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
