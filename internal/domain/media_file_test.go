package domain

import "testing"

func validMediaFile() MediaFile {
	return MediaFile{
		ID:     "mf1",
		ItemID: "i1",
		Path:   "/media/music/fleetwood-mac/rumours/01-second-hand-news.flac",
	}
}

func TestMediaFile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*MediaFile)
		wantErr bool
	}{
		{"valid", func(_ *MediaFile) {}, false},
		{"missing ID", func(m *MediaFile) { m.ID = "" }, true},
		{"missing ItemID", func(m *MediaFile) { m.ItemID = "" }, true},
		{"missing Path", func(m *MediaFile) { m.Path = "" }, true},
		{"no hashes is valid (computed later)", func(m *MediaFile) {
			m.OSHash, m.MD5, m.SHA1 = "", "", ""
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMediaFile()
			tt.mutate(&m)
			err := m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
