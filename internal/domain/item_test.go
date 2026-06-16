package domain

import "testing"

func TestItem_HasFile(t *testing.T) {
	item := &Item{}
	if item.HasFile() {
		t.Error("HasFile() = true for nil MediaFile, want false")
	}
	item.MediaFile = &MediaFile{Path: "/media/test.mp4"}
	if !item.HasFile() {
		t.Error("HasFile() = false for non-nil MediaFile, want true")
	}
}

func TestItem_IsWanted(t *testing.T) {
	cases := []struct {
		name      string
		monitored bool
		status    ItemStatus
		want      bool
	}{
		{"monitored+wanted", true, StatusWanted, true},
		{"not monitored+wanted", false, StatusWanted, false},
		{"monitored+imported", true, StatusImported, false},
		{"monitored+grabbed", true, StatusGrabbed, false},
		{"monitored+missing", true, StatusMissing, false},
		{"monitored+skipped", true, StatusSkipped, false},
		{"monitored+downloading", true, StatusDownloading, false},
		{"not monitored+imported", false, StatusImported, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := &Item{Monitored: tc.monitored, Status: tc.status}
			if got := item.IsWanted(); got != tc.want {
				t.Errorf("IsWanted() = %v, want %v", got, tc.want)
			}
		})
	}
}
