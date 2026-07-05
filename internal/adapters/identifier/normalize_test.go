package identifier

import (
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"basic lowercase", "Take It on the Run", "take it on the run"},
		{"remaster paren", "Take It on the Run (2024 Remaster)", "take it on the run"},
		{"remastered paren", "Take It on the Run (Remastered)", "take it on the run"},
		{"deluxe paren", "Bella Donna (Deluxe Edition)", "bella donna"},
		{"explicit paren", "Song Title (Explicit)", "song title"},
		{"live paren", "Keep On Loving You (Live)", "keep on loving you"},
		{"remix paren", "Keep On Loving You (Club Remix)", "keep on loving you"},
		{"version paren", "Song (Radio Version)", "song"},
		{"clean paren", "Song (Clean)", "song"},
		{"bracket edition", "Thriller [Deluxe Edition]", "thriller"},
		{"bracket remaster", "Thriller [2009 Remaster]", "thriller"},
		{"feat dot", "Take It on the Run feat. Sara Bareilles", "take it on the run"},
		{"ft dot", "Take It on the Run ft. Sara Bareilles", "take it on the run"},
		{"featuring", "Take It on the Run featuring Sara Bareilles", "take it on the run"},
		{"featuring mixed case", "Take It on the Run Featuring Sara Bareilles", "take it on the run"},
		{"unicode nfc", "Café de Flore", "café de flore"},
		{"multi whitespace", "Take  It   On", "take it on"},
		{"no qualifiers", "Hi Infidelity", "hi infidelity"},
		{"already normalized", "take it on the run", "take it on the run"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if got2 := normalizeTitle(got); got2 != got {
				t.Errorf("normalizeTitle not idempotent: second pass %q → %q", got, got2)
			}
		})
	}
}

func TestNormalizeArtist(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"strip the", "The Beatles", "beatles"},
		{"strip a", "A Flock of Seagulls", "flock of seagulls"},
		{"strip an", "An Artist", "artist"},
		{"no article", "REO Speedwagon", "reo speedwagon"},
		{"mid-word the", "Theater", "theater"},
		{"mid-word a", "Arcade Fire", "arcade fire"},
		{"the not standalone", "Them Crooked Vultures", "them crooked vultures"},
		{"unicode nfc", "Bjørk", "bjørk"},
		{"multi whitespace", "REO  Speedwagon", "reo speedwagon"},
		{"already normalized", "beatles", "beatles"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeArtist(tt.input)
			if got != tt.want {
				t.Errorf("normalizeArtist(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if got2 := normalizeArtist(got); got2 != got {
				t.Errorf("normalizeArtist not idempotent: second pass %q → %q", got, got2)
			}
		})
	}
}

func TestNormalizeAlbum(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no qualifiers", "Hi Infidelity", "hi infidelity"},
		{"remaster paren", "Hi Infidelity (2024 Remaster)", "hi infidelity"},
		{"remastered paren", "Hi Infidelity (Remastered)", "hi infidelity"},
		{"deluxe paren", "Thriller (Deluxe Edition)", "thriller"},
		{"bracket edition", "Thriller [Deluxe Edition]", "thriller"},
		{"bracket remaster", "Dark Side of the Moon [2023 Remaster]", "dark side of the moon"},
		{"unicode", "Ágætis byrjun", "ágætis byrjun"},
		{"already normalized", "hi infidelity", "hi infidelity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAlbum(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAlbum(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if got2 := normalizeAlbum(got); got2 != got {
				t.Errorf("normalizeAlbum not idempotent: second pass %q → %q", got, got2)
			}
		})
	}
}

func TestAlbumTagSimilarity(t *testing.T) {
	tests := []struct {
		name         string
		embedded     string
		releaseTitle string
		want         float64
	}{
		{"exact match", "Hi Infidelity", "Hi Infidelity", 1.00},
		{"exact case insensitive", "hi infidelity", "Hi Infidelity", 1.00},
		{"prefix — remaster paren", "Hi Infidelity", "Hi Infidelity (2024 Remaster)", 0.85},
		{"prefix — deluxe paren", "Thriller", "Thriller (Deluxe Edition)", 0.85},
		{"prefix — bracket remaster", "Dark Side", "Dark Side [2023 Remaster]", 0.85},
		{"contains — compilation", "Hi Infidelity", "The Best of Hi Infidelity", 0.60},
		{"no overlap", "Hi Infidelity", "Find Your Own Way Home", 0.00},
		{"empty embedded", "", "Hi Infidelity", 0.00},
		{"empty release", "Hi Infidelity", "", 0.00},
		{"both empty", "", "", 0.00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := albumTagSimilarity(tt.embedded, tt.releaseTitle)
			if got != tt.want {
				t.Errorf("albumTagSimilarity(%q, %q) = %.2f, want %.2f", tt.embedded, tt.releaseTitle, got, tt.want)
			}
		})
	}

	// T2 invariant: exact > prefix > no match
	t.Run("T2 exact > prefix > no match", func(t *testing.T) {
		exact := albumTagSimilarity("Hi Infidelity", "Hi Infidelity")
		prefix := albumTagSimilarity("Hi Infidelity", "Hi Infidelity (2024 Remaster)")
		none := albumTagSimilarity("Hi Infidelity", "Find Your Own Way Home")
		if !(exact > prefix && prefix > none) {
			t.Errorf("T2 violated: exact=%.2f prefix=%.2f none=%.2f", exact, prefix, none)
		}
	})
}

func TestDurationScore(t *testing.T) {
	tests := []struct {
		name          string
		embeddedMS    int
		candidateSecs int
		want          float64
	}{
		{"diff 0s", 3000, 3, 1.0},
		{"diff 1s", 4000, 3, 1.0},
		{"diff 2s", 5000, 3, 0.75},
		{"diff 3s", 6000, 3, 0.75},
		{"diff 4s", 7000, 3, 0.50},
		{"diff 5s", 8000, 3, 0.50},
		{"diff 6s", 9000, 3, 0.25},
		{"diff 10s", 13000, 3, 0.25},
		{"diff 11s", 14000, 3, 0.0},
		{"diff 60s", 63000, 3, 0.0},
		{"diff 1s reverse", 3000, 4, 1.0},
		{"diff 11s reverse", 3000, 14, 0.0},
		{"zero embedded", 0, 180, 0.0},
		{"zero candidate", 3000, 0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationScore(tt.embeddedMS, tt.candidateSecs)
			if got != tt.want {
				t.Errorf("durationScore(%d, %d) = %.2f, want %.2f", tt.embeddedMS, tt.candidateSecs, got, tt.want)
			}
		})
	}
}
