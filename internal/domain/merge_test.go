package domain_test

import (
	"purser/internal/domain"
	"testing"
)

func TestMergeExternalItems(t *testing.T) {
	cases := []struct {
		name    string
		input   []domain.SourcedExternalItem
		want    *domain.ExternalItem
		wantNil bool
	}{
		{
			name:    "empty input returns nil",
			input:   []domain.SourcedExternalItem{},
			wantNil: true,
		},
		{
			name: "single provider returns its data unchanged",
			input: []domain.SourcedExternalItem{
				{
					Source: "musicbrainz",
					Item: &domain.ExternalItem{
						Title:    "Radiohead",
						Overview: "A band from Oxford",
						Year:     1985,
						Tags:     []string{"rock", "alternative"},
					},
				},
			},
			want: &domain.ExternalItem{
				Title:    "Radiohead",
				Overview: "A band from Oxford",
				Year:     1985,
				Tags:     []string{"rock", "alternative"},
			},
		},
		{
			name: "scalar fill-in: primary empty overview filled by secondary",
			input: []domain.SourcedExternalItem{
				{
					Source: "musicbrainz",
					Item: &domain.ExternalItem{
						Title:    "Radiohead",
						Overview: "",
					},
				},
				{
					Source: "lastfm",
					Item: &domain.ExternalItem{
						Title:    "Radiohead",
						Overview: "English rock band formed in Abingdon",
					},
				},
			},
			want: &domain.ExternalItem{
				Title:    "Radiohead",
				Overview: "English rock band formed in Abingdon",
			},
		},
		{
			name: "scalar conflict: primary year wins over secondary",
			input: []domain.SourcedExternalItem{
				{
					Source: "musicbrainz",
					Item: &domain.ExternalItem{
						Title: "Radiohead",
						Year:  1985,
					},
				},
				{
					Source: "lastfm",
					Item: &domain.ExternalItem{
						Title: "Radiohead",
						Year:  1991,
					},
				},
			},
			want: &domain.ExternalItem{
				Title: "Radiohead",
				Year:  1985,
			},
		},
		{
			name: "image union: deduplicates by URL and stamps source",
			input: []domain.SourcedExternalItem{
				{
					Source: "musicbrainz",
					Item: &domain.ExternalItem{
						Images: []domain.ExternalImage{
							{Type: domain.ImageTypeThumbnail, URL: "https://example.com/a.jpg"},
							{Type: domain.ImageTypeBanner, URL: "https://example.com/b.jpg"},
						},
					},
				},
				{
					Source: "fanart",
					Item: &domain.ExternalItem{
						Images: []domain.ExternalImage{
							{Type: domain.ImageTypeBanner, URL: "https://example.com/b.jpg"}, // duplicate
							{Type: domain.ImageTypeBackground, URL: "https://example.com/c.jpg"},
						},
					},
				},
			},
			want: &domain.ExternalItem{
				Images: []domain.ExternalImage{
					{Type: domain.ImageTypeThumbnail, URL: "https://example.com/a.jpg", Source: "musicbrainz"},
					{Type: domain.ImageTypeBanner, URL: "https://example.com/b.jpg", Source: "musicbrainz"},
					{Type: domain.ImageTypeBackground, URL: "https://example.com/c.jpg", Source: "fanart"},
				},
			},
		},
		{
			name: "tag union: deduplicates and preserves stable order",
			input: []domain.SourcedExternalItem{
				{
					Source: "musicbrainz",
					Item: &domain.ExternalItem{
						Tags: []string{"rock", "alternative"},
					},
				},
				{
					Source: "lastfm",
					Item: &domain.ExternalItem{
						Tags: []string{"alternative", "indie", "experimental"},
					},
				},
			},
			want: &domain.ExternalItem{
				Tags: []string{"rock", "alternative", "indie", "experimental"},
			},
		},
		{
			name: "ExternalID map merge: primary wins on collision, secondary adds unique keys",
			input: []domain.SourcedExternalItem{
				{
					Source: "musicbrainz",
					Item: &domain.ExternalItem{
						ExternalIDs: map[string]string{
							"mbid":     "a74b1b7f-71a5-4011-9441-d0b5e4122711",
							"wikidata": "Q154026",
						},
					},
				},
				{
					Source: "lastfm",
					Item: &domain.ExternalItem{
						ExternalIDs: map[string]string{
							"mbid":   "wrong-id", // collision — primary must win
							"lastfm": "Radiohead",
						},
					},
				},
			},
			want: &domain.ExternalItem{
				ExternalIDs: map[string]string{
					"mbid":     "a74b1b7f-71a5-4011-9441-d0b5e4122711",
					"wikidata": "Q154026",
					"lastfm":   "Radiohead",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.MergeExternalItems(tc.input)

			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil result, got nil")
			}

			assertExternalItemEqual(t, tc.want, got)
		})
	}
}

func assertExternalItemEqual(t *testing.T, want, got *domain.ExternalItem) {
	t.Helper()

	if want.Title != got.Title {
		t.Errorf("Title: want %q, got %q", want.Title, got.Title)
	}
	if want.Overview != got.Overview {
		t.Errorf("Overview: want %q, got %q", want.Overview, got.Overview)
	}
	if want.Year != got.Year {
		t.Errorf("Year: want %d, got %d", want.Year, got.Year)
	}

	if len(want.Tags) != len(got.Tags) {
		t.Errorf("Tags: want %v, got %v", want.Tags, got.Tags)
	} else {
		for i, tag := range want.Tags {
			if got.Tags[i] != tag {
				t.Errorf("Tags[%d]: want %q, got %q", i, tag, got.Tags[i])
			}
		}
	}

	if len(want.Images) != len(got.Images) {
		t.Errorf("Images: want %d images, got %d; want %v, got %v",
			len(want.Images), len(got.Images), want.Images, got.Images)
	} else {
		for i, img := range want.Images {
			if got.Images[i].URL != img.URL {
				t.Errorf("Images[%d].URL: want %q, got %q", i, img.URL, got.Images[i].URL)
			}
			if got.Images[i].Type != img.Type {
				t.Errorf("Images[%d].Type: want %q, got %q", i, img.Type, got.Images[i].Type)
			}
			if got.Images[i].Source != img.Source {
				t.Errorf("Images[%d].Source: want %q, got %q", i, img.Source, got.Images[i].Source)
			}
		}
	}

	if len(want.ExternalIDs) != len(got.ExternalIDs) {
		t.Errorf("ExternalIDs: want %v, got %v", want.ExternalIDs, got.ExternalIDs)
	} else {
		for k, wantVal := range want.ExternalIDs {
			if gotVal, ok := got.ExternalIDs[k]; !ok || gotVal != wantVal {
				t.Errorf("ExternalIDs[%q]: want %q, got %q (present=%v)", k, wantVal, gotVal, ok)
			}
		}
	}
}
