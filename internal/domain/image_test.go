package domain_test

import (
	"purser/internal/domain"
	"testing"
)

func TestApplicableImageTypes(t *testing.T) {
	cases := []struct {
		contentType string
		entityType  string
		want        []domain.ImageType
	}{
		// library_entry
		{"music", "library_entry", []domain.ImageType{domain.ImageTypeHero, domain.ImageTypeBanner, domain.ImageTypeThumbnail}},
		{"tv", "library_entry", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner, domain.ImageTypeHero, domain.ImageTypeBackground}},
		{"adult", "library_entry", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner}},
		{"movie", "library_entry", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner, domain.ImageTypeBackground}},
		{"jav", "library_entry", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner}},
		{"book", "library_entry", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner}},
		// group
		{"music", "group", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner}},
		{"tv", "group", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner}},
		{"adult", "group", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeBanner}},
		// item
		{"music", "item", []domain.ImageType{domain.ImageTypeThumbnail}},
		{"tv", "item", []domain.ImageType{domain.ImageTypeThumbnail}},
		{"adult", "item", []domain.ImageType{domain.ImageTypeThumbnail, domain.ImageTypeBanner}},
		{"movie", "item", []domain.ImageType{domain.ImageTypePoster, domain.ImageTypeThumbnail}},
		{"jav", "item", []domain.ImageType{domain.ImageTypeThumbnail, domain.ImageTypeBanner}},
		// person — content type is ignored
		{"music", "person", []domain.ImageType{domain.ImageTypeHero, domain.ImageTypeBanner, domain.ImageTypeThumbnail}},
		{"tv", "person", []domain.ImageType{domain.ImageTypeHero, domain.ImageTypeBanner, domain.ImageTypeThumbnail}},
		{"", "person", []domain.ImageType{domain.ImageTypeHero, domain.ImageTypeBanner, domain.ImageTypeThumbnail}},
		// unrecognized combinations → empty slice
		{"", "", []domain.ImageType{}},
		{"unknown", "unknown", []domain.ImageType{}},
		{"movie", "unknown", []domain.ImageType{}},
	}

	for _, tc := range cases {
		t.Run(tc.entityType+"/"+tc.contentType, func(t *testing.T) {
			got := domain.ApplicableImageTypes(tc.contentType, tc.entityType)
			if len(got) != len(tc.want) {
				t.Fatalf("ApplicableImageTypes(%q, %q) len=%d, want %d; got %v",
					tc.contentType, tc.entityType, len(got), len(tc.want), got)
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("ApplicableImageTypes(%q, %q)[%d] = %q, want %q",
						tc.contentType, tc.entityType, i, v, tc.want[i])
				}
			}
		})
	}
}
