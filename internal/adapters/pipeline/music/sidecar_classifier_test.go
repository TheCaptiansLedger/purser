package music_test

import (
	"context"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func TestSidecarClassifier_ContentTypes(t *testing.T) {
	c := music.SidecarClassifier{}
	got := c.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestSidecarClassifier_Classify(t *testing.T) {
	tests := []struct {
		path string
		want ports.SidecarKind
	}{
		{path: "/music/artist/album/01.flac", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.mp3", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.M4A", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.ogg", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.wav", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.aac", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.wma", want: ports.SidecarKindNone},
		{path: "/music/artist/album/01.opus", want: ports.SidecarKindNone},
		{path: "/music/artist/album/cover.jpg", want: ports.SidecarKindImage},
		{path: "/music/artist/album/cover.JPEG", want: ports.SidecarKindImage},
		{path: "/music/artist/album/folder.png", want: ports.SidecarKindImage},
		{path: "/music/artist/album/front.webp", want: ports.SidecarKindImage},
		{path: "/music/artist/album/back.gif", want: ports.SidecarKindImage},
		{path: "/music/artist/album/album.nfo", want: ports.SidecarKindOther},
		{path: "/music/artist/album/album.cue", want: ports.SidecarKindOther},
		{path: "/music/artist/album/rip.log", want: ports.SidecarKindOther},
		{path: "/music/artist/album/playlist.m3u", want: ports.SidecarKindOther},
		{path: "/music/artist/album/Thumbs.db", want: ports.SidecarKindOther},
		{path: "/music/artist/album/noextension", want: ports.SidecarKindOther},
	}

	c := music.SidecarClassifier{}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := c.Classify(context.Background(), tt.path)
			if err != nil {
				t.Fatalf("Classify(%q) returned error: %v", tt.path, err)
			}
			if got != tt.want {
				t.Errorf("Classify(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
