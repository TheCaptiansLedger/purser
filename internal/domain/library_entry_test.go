package domain

import "testing"

func validLibraryEntry() LibraryEntry {
	return LibraryEntry{
		ID:          "le1",
		ContentType: ContentTypeMusic,
		Kind:        KindArtist,
		Name:        "Fleetwood Mac",
		MonitorMode: MonitorModeAll,
	}
}

func TestLibraryEntry_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*LibraryEntry)
		wantErr bool
	}{
		{"valid", func(_ *LibraryEntry) {}, false},
		{"missing ID", func(l *LibraryEntry) { l.ID = "" }, true},
		{"missing ContentType", func(l *LibraryEntry) { l.ContentType = "" }, true},
		{"missing Kind", func(l *LibraryEntry) { l.Kind = "" }, true},
		{"any non-empty Kind is valid (open enum)", func(l *LibraryEntry) {
			l.Kind = Kind("collection")
		}, false},
		{"missing Name", func(l *LibraryEntry) { l.Name = "" }, true},
		{"missing MonitorMode", func(l *LibraryEntry) { l.MonitorMode = "" }, true},
		{"invalid MonitorMode", func(l *LibraryEntry) { l.MonitorMode = "sometimes" }, true},
		{"no ParentID is valid (top-level entry)", func(l *LibraryEntry) { l.ParentID = "" }, false},
		{"with ParentID is valid (nested entry)", func(l *LibraryEntry) { l.ParentID = "network1" }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := validLibraryEntry()
			tt.mutate(&l)
			err := l.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
