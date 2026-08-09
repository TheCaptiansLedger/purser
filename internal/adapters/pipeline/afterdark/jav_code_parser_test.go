package afterdark_test

import (
	"purser/internal/adapters/pipeline/afterdark"
	"testing"
)

func TestExtractJAVCode(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode string
		wantOK   bool
	}{
		{
			name:     "hyphen-separated code, uppercase",
			path:     "/jav/SSIS-001.mp4",
			wantCode: "SSIS-001",
			wantOK:   true,
		},
		{
			name:     "hyphen-separated code, lowercase",
			path:     "/jav/ssis-001.mp4",
			wantCode: "SSIS-001",
			wantOK:   true,
		},
		{
			name:     "underscore-separated code",
			path:     "/jav/STARS_123.mp4",
			wantCode: "STARS-123",
			wantOK:   true,
		},
		{
			name:     "code embedded in a longer filename",
			path:     "/jav/[HD] MIDE-045 Some Title 1080p.mp4",
			wantCode: "MIDE-045",
			wantOK:   true,
		},
		{
			name:     "concatenated code, no separator",
			path:     "/jav/ssis00001.mp4",
			wantCode: "SSIS-00001",
			wantOK:   true,
		},
		{
			name:     "leaf has no code, falls back to parent folder name",
			path:     "/jav/SSIS-001/scene.mp4",
			wantCode: "SSIS-001",
			wantOK:   true,
		},
		{
			name:     "no code anywhere: no guess",
			path:     "/scenes/Brazzers - Sneaking Into My Roommate.mp4",
			wantCode: "",
			wantOK:   false,
		},
		{
			name:     "short digit run after a space is not treated as a code",
			path:     "/scenes/Scene 22.mp4",
			wantCode: "",
			wantOK:   false,
		},
		{
			name:     "resolution/quality tags are not treated as a code",
			path:     "/scenes/My Favorite Scene 1080p.mp4",
			wantCode: "",
			wantOK:   false,
		},
		{
			name:     "year alone is not treated as a code",
			path:     "/scenes/Best Of 2023.mp4",
			wantCode: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotOK := afterdark.ExtractJAVCode(tt.path)
			if gotCode != tt.wantCode || gotOK != tt.wantOK {
				t.Fatalf("ExtractJAVCode(%q) = (%q, %v), want (%q, %v)",
					tt.path, gotCode, gotOK, tt.wantCode, tt.wantOK)
			}
		})
	}
}
