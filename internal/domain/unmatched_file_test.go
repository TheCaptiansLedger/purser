package domain

import "testing"

func validUnmatchedFile() UnmatchedFile {
	return UnmatchedFile{
		ID:          "uf1",
		Path:        "/media/incoming/new-arrivals/track01.flac",
		ContentType: ContentTypeMusic,
		GroupKey:    "/media/incoming/new-arrivals/track01.flac",
		Status:      UnmatchedFileStatusPending,
	}
}

func TestUnmatchedFile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*UnmatchedFile)
		wantErr bool
	}{
		{"valid", func(_ *UnmatchedFile) {}, false},
		{"missing ID", func(u *UnmatchedFile) { u.ID = "" }, true},
		{"missing Path", func(u *UnmatchedFile) { u.Path = "" }, true},
		{"missing ContentType is valid (an unconfigured scan root)", func(u *UnmatchedFile) { u.ContentType = "" }, false},
		{"missing GroupKey", func(u *UnmatchedFile) { u.GroupKey = "" }, true},
		{"missing Status", func(u *UnmatchedFile) { u.Status = "" }, true},
		{"invalid Status", func(u *UnmatchedFile) { u.Status = "bogus" }, true},
		{"Status matched is valid", func(u *UnmatchedFile) { u.Status = UnmatchedFileStatusMatched }, false},
		{"Status dismissed is valid", func(u *UnmatchedFile) { u.Status = UnmatchedFileStatusDismissed }, false},
		{"no hashes is valid (computed later)", func(u *UnmatchedFile) {
			u.OSHash, u.MD5, u.SHA1, u.SHA512 = "", "", "", ""
		}, false},
		{"vinyl side-lettered TrackNumber is valid, not coerced", func(u *UnmatchedFile) { u.TrackNumber = "A1" }, false},
		{"nil Fingerprint/Candidates is valid (computed later)", func(u *UnmatchedFile) {
			u.Fingerprint, u.Candidates = nil, nil
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := validUnmatchedFile()
			tt.mutate(&u)
			err := u.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
