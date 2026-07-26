package service_test

import (
	"context"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakeFilenameParser is a minimal ports.FilenameParser double for exercising
// FilenameParserRegistry's dispatch — every call returns a fixed,
// recognizable (artist, album, ok) so tests can tell it apart from
// service.NoopFilenameParser's always-ok=false behavior.
type fakeFilenameParser struct {
	contentTypes []domain.ContentType
	artist       string
	album        string
	ok           bool
}

func (f *fakeFilenameParser) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeFilenameParser) Parse(_ context.Context, _, _ string) (artist, album string, ok bool) {
	return f.artist, f.album, f.ok
}

func TestFilenameParserRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakeFilenameParser{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		artist:       "Fixed Artist",
		album:        "Fixed Album",
		ok:           true,
	}
	registry := service.NewFilenameParserRegistry(music)

	gotArtist, gotAlbum, gotOK := registry.Parse(context.Background(), domain.ContentTypeMusic, "/music/Fixed Artist/Fixed Album", "/music")
	if gotArtist != "Fixed Artist" || gotAlbum != "Fixed Album" || !gotOK {
		t.Fatalf("Parse() = (%q, %q, %v), want (%q, %q, %v)", gotArtist, gotAlbum, gotOK, "Fixed Artist", "Fixed Album", true)
	}
}

func TestFilenameParserRegistry_FallsBackToNoopWhenUnregistered(t *testing.T) {
	music := &fakeFilenameParser{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, artist: "x", album: "y", ok: true}
	registry := service.NewFilenameParserRegistry(music)

	gotArtist, gotAlbum, gotOK := registry.Parse(context.Background(), domain.ContentTypeMovie, "/movies/Some Movie (2020)", "/movies")
	if gotArtist != "" || gotAlbum != "" || gotOK {
		t.Fatalf("Parse() = (%q, %q, %v), want (\"\", \"\", false) (NoopFilenameParser fallback)", gotArtist, gotAlbum, gotOK)
	}
}

func TestFilenameParserRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewFilenameParserRegistry()

	gotArtist, gotAlbum, gotOK := registry.Parse(context.Background(), domain.ContentTypeMusic, "/music/Artist/Album", "/music")
	if gotArtist != "" || gotAlbum != "" || gotOK {
		t.Fatalf("Parse() = (%q, %q, %v), want (\"\", \"\", false)", gotArtist, gotAlbum, gotOK)
	}
}

func TestNoopFilenameParser_AlwaysReturnsNoGuess(t *testing.T) {
	p := service.NoopFilenameParser{}

	gotArtist, gotAlbum, gotOK := p.Parse(context.Background(), "/music/Artist/Album", "/music")
	if gotArtist != "" || gotAlbum != "" || gotOK {
		t.Fatalf("Parse() = (%q, %q, %v), want (\"\", \"\", false)", gotArtist, gotAlbum, gotOK)
	}
}

func TestNoopFilenameParser_ContentTypesReturnsNil(t *testing.T) {
	p := service.NoopFilenameParser{}
	if got := p.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
