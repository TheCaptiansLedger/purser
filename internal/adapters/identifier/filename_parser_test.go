package identifier_test

import (
	"purser/internal/adapters/identifier"
	"testing"
)

func TestParseVideoFilename_MovieTitleYear(t *testing.T) {
	tests := []struct {
		input   string
		title   string
		year    int
		season  int
		episode int
	}{
		{"The.Matrix.1999.BluRay.1080p", "The Matrix", 1999, 0, 0},
		{"Inception (2010)", "Inception", 2010, 0, 0},
		{"The Dark Knight 2008 BluRay", "The Dark Knight", 2008, 0, 0},
		{"No.Country.for.Old.Men.2007", "No Country for Old Men", 2007, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			title, year, season, episode := identifier.ParseVideoFilename(tc.input)
			if title != tc.title {
				t.Errorf("title = %q, want %q", title, tc.title)
			}
			if year != tc.year {
				t.Errorf("year = %d, want %d", year, tc.year)
			}
			if season != tc.season {
				t.Errorf("season = %d, want %d", season, tc.season)
			}
			if episode != tc.episode {
				t.Errorf("episode = %d, want %d", episode, tc.episode)
			}
		})
	}
}

func TestParseVideoFilename_TV(t *testing.T) {
	tests := []struct {
		input   string
		title   string
		season  int
		episode int
	}{
		{"Breaking.Bad.S01E01.720p", "Breaking Bad", 1, 1},
		{"Game.of.Thrones.S08E06.WEB-DL", "Game of Thrones", 8, 6},
		{"The.Wire.1x01.DVDRip", "The Wire", 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			title, _, season, episode := identifier.ParseVideoFilename(tc.input)
			if title != tc.title {
				t.Errorf("title = %q, want %q", title, tc.title)
			}
			if season != tc.season {
				t.Errorf("season = %d, want %d", season, tc.season)
			}
			if episode != tc.episode {
				t.Errorf("episode = %d, want %d", episode, tc.episode)
			}
		})
	}
}

func TestParseTrackFilename(t *testing.T) {
	tests := []struct {
		input    string
		trackNum int
		title    string
	}{
		{"01 - Bella Donna", 1, "Bella Donna"},
		{"03. Gold Dust Woman", 3, "Gold Dust Woman"},
		{"12 Sara", 12, "Sara"},
		{"Untitled Track", 0, "Untitled Track"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			n, title := identifier.ParseTrackFilename(tc.input)
			if n != tc.trackNum {
				t.Errorf("trackNum = %d, want %d", n, tc.trackNum)
			}
			if title != tc.title {
				t.Errorf("title = %q, want %q", title, tc.title)
			}
		})
	}
}

func TestParseAdultFilename_StudioDateTitle(t *testing.T) {
	studio, title, _, date, fieldCount := identifier.ParseAdultFilename("SOD 2024-03-15 Amazing Scene Title")
	if studio != "SOD" {
		t.Errorf("studio = %q, want SOD", studio)
	}
	if title == "" {
		t.Error("expected non-empty title")
	}
	if date.IsZero() {
		t.Error("expected non-zero date")
	}
	if fieldCount < 3 {
		t.Errorf("fieldCount = %d, want >= 3", fieldCount)
	}
}

func TestParseAdultFilename_TitleOnly(t *testing.T) {
	_, title, _, _, fieldCount := identifier.ParseAdultFilename("just-a-scene-title")
	if title == "" {
		t.Error("expected non-empty title")
	}
	if fieldCount >= 3 {
		t.Errorf("fieldCount = %d, want < 3 for title-only", fieldCount)
	}
}
