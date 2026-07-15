package music_test

import (
	"errors"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"testing"
)

func validRelease() *music.Release {
	return &music.Release{
		ID:             "r1",
		GroupID:        "group1",
		LibraryEntryID: "entry1",
		Title:          "Hi Infidelity (2024 Remaster)",
		Status:         music.ReleaseStatusStub,
	}
}

func TestRelease_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*music.Release)
		wantErr bool
	}{
		{
			name:    "valid release with only required fields set",
			mutate:  func(*music.Release) {},
			wantErr: false,
		},
		{
			name: "valid release with every field populated",
			mutate: func(r *music.Release) {
				r.Country = "US"
				r.Label = "CBS"
				r.CatalogNumber = "FZ 38079"
				r.Barcode = "07464538792"
				r.Format = "CD"
				r.MediumCount = 1
				r.TrackCount = 10
				r.IsDefault = true
				r.Monitored = true
				r.Status = music.ReleaseStatusImported
				r.MBID = "89ad4ac3-39f7-470e-963a-56509c546377"
			},
			wantErr: false,
		},
		{name: "missing ID is invalid", mutate: func(r *music.Release) { r.ID = "" }, wantErr: true},
		{name: "missing GroupID is invalid", mutate: func(r *music.Release) { r.GroupID = "" }, wantErr: true},
		{name: "missing LibraryEntryID is invalid", mutate: func(r *music.Release) { r.LibraryEntryID = "" }, wantErr: true},
		{name: "missing Title is invalid", mutate: func(r *music.Release) { r.Title = "" }, wantErr: true},
		{name: "missing Status is invalid", mutate: func(r *music.Release) { r.Status = "" }, wantErr: true},
		{name: "unknown Status is invalid", mutate: func(r *music.Release) { r.Status = "released" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRelease()
			tt.mutate(r)

			err := r.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestRelease_Validate_ReturnsDomainValidationError(t *testing.T) {
	r := validRelease()
	r.Title = ""

	var verr *domain.ValidationError
	err := r.Validate()
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() returned %v, want *domain.ValidationError", err)
	}
	if len(verr.Errors) != 1 || verr.Errors[0].Field != "Title" {
		t.Fatalf("Validate() returned %+v, want a single Title field error", verr)
	}
}
