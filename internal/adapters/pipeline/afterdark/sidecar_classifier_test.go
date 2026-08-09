package afterdark_test

import (
	"context"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func TestSidecarClassifier_ContentTypes(t *testing.T) {
	c := afterdark.SidecarClassifier{}
	got := c.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

func TestSidecarClassifier_Classify(t *testing.T) {
	tests := []struct {
		path string
		want ports.SidecarKind
	}{
		{path: "/scenes/studio/scene.mp4", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.MKV", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.avi", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.wmv", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.mov", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.m4v", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.webm", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.flv", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.ts", want: ports.SidecarKindNone},
		{path: "/scenes/studio/scene.m2ts", want: ports.SidecarKindNone},
		{path: "/scenes/studio/poster.jpg", want: ports.SidecarKindImage},
		{path: "/scenes/studio/poster.JPEG", want: ports.SidecarKindImage},
		{path: "/scenes/studio/screenshot.png", want: ports.SidecarKindImage},
		{path: "/scenes/studio/cover.webp", want: ports.SidecarKindImage},
		{path: "/scenes/studio/back.gif", want: ports.SidecarKindImage},
		{path: "/scenes/studio/scene.nfo", want: ports.SidecarKindOther},
		{path: "/scenes/studio/scene-trailer.mp4", want: ports.SidecarKindOther},
		{path: "/scenes/studio/Scene_Trailer.MKV", want: ports.SidecarKindOther},
		{path: "/scenes/studio/trailer.mp4", want: ports.SidecarKindOther},
		{path: "/scenes/studio/TRAILER.avi", want: ports.SidecarKindOther},
		{path: "/scenes/studio/Blazing Trailers.mp4", want: ports.SidecarKindNone},
		{path: "/scenes/studio/Thumbs.db", want: ports.SidecarKindOther},
		{path: "/scenes/studio/noextension", want: ports.SidecarKindOther},
	}

	c := afterdark.SidecarClassifier{}
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
