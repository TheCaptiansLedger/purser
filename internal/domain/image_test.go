package domain

import "testing"

func validImage() Image {
	return Image{
		ID:        "img1",
		OwnerType: "person",
		OwnerID:   "p1",
		ImageType: ImageTypeHero,
		URL:       "https://example.com/p1.jpg",
	}
}

func TestImage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Image)
		wantErr bool
	}{
		{"valid", func(_ *Image) {}, false},
		{"missing ID", func(i *Image) { i.ID = "" }, true},
		{"missing OwnerType", func(i *Image) { i.OwnerType = "" }, true},
		{"missing OwnerID", func(i *Image) { i.OwnerID = "" }, true},
		{"missing ImageType", func(i *Image) { i.ImageType = "" }, true},
		{"missing URL", func(i *Image) { i.URL = "" }, true},
		{"module-owned OwnerType is valid (open string)", func(i *Image) {
			i.OwnerType = "afterdark.performer_profile"
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := validImage()
			tt.mutate(&img)
			err := img.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
