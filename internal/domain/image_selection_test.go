package domain

import "testing"

func validImageSelection() ImageSelection {
	return ImageSelection{
		OwnerType: "person",
		OwnerID:   "p1",
		ImageType: ImageTypeHero,
		ImageID:   "img1",
	}
}

func TestImageSelection_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ImageSelection)
		wantErr bool
	}{
		{"valid", func(_ *ImageSelection) {}, false},
		{"missing OwnerType", func(s *ImageSelection) { s.OwnerType = "" }, true},
		{"missing OwnerID", func(s *ImageSelection) { s.OwnerID = "" }, true},
		{"missing ImageType", func(s *ImageSelection) { s.ImageType = "" }, true},
		{"missing ImageID", func(s *ImageSelection) { s.ImageID = "" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sel := validImageSelection()
			tt.mutate(&sel)
			err := sel.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
