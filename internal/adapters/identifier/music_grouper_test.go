package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func musicFile(path string) domain.ScannedFile {
	return domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
		Size:        4096,
	}
}

func nonMusicFile(path string) domain.ScannedFile {
	return domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeAdult,
		Size:        4096,
	}
}

func TestMusicFolderGrouper_SingleDisc(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	files := []domain.ScannedFile{
		musicFile("/music/Hi Infidelity/01 - Don't Let Him Go.flac"),
		musicFile("/music/Hi Infidelity/02 - Keep On Loving You.flac"),
		musicFile("/music/Hi Infidelity/03 - Follow My Heart.flac"),
	}

	groups, err := g.Group(context.Background(), files)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if groups[0].RootPath != "/music/Hi Infidelity" {
		t.Errorf("RootPath = %q, want %q", groups[0].RootPath, "/music/Hi Infidelity")
	}
	if len(groups[0].Files) != 3 {
		t.Errorf("group files = %d, want 3", len(groups[0].Files))
	}
}

func TestMusicFolderGrouper_MultiDisc_CD(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	files := []domain.ScannedFile{
		musicFile("/music/The Wall/CD1/01 - In the Flesh.flac"),
		musicFile("/music/The Wall/CD1/02 - The Thin Ice.flac"),
		musicFile("/music/The Wall/CD2/01 - Hey You.flac"),
		musicFile("/music/The Wall/CD2/02 - Is There Anybody Out There.flac"),
	}

	groups, err := g.Group(context.Background(), files)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1 (multi-disc should be merged)", len(groups))
	}
	want := "/music/The Wall"
	if groups[0].RootPath != want {
		t.Errorf("RootPath = %q, want %q", groups[0].RootPath, want)
	}
	if len(groups[0].Files) != 4 {
		t.Errorf("group files = %d, want 4", len(groups[0].Files))
	}
}

func TestMusicFolderGrouper_MultiDisc_DiscN(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	files := []domain.ScannedFile{
		musicFile("/music/Sandinista/Disc 1/01 - The Magnificent Seven.flac"),
		musicFile("/music/Sandinista/Disc 2/01 - Ivan Meets G.I. Joe.flac"),
		musicFile("/music/Sandinista/Disc 3/01 - Hitsville U.K..flac"),
	}

	groups, err := g.Group(context.Background(), files)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1 (3-disc should be merged)", len(groups))
	}
	if groups[0].RootPath != "/music/Sandinista" {
		t.Errorf("RootPath = %q, want %q", groups[0].RootPath, "/music/Sandinista")
	}
	if len(groups[0].Files) != 3 {
		t.Errorf("group files = %d, want 3", len(groups[0].Files))
	}
}

func TestMusicFolderGrouper_TwoAlbums_TwoGroups(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	files := []domain.ScannedFile{
		musicFile("/music/Hi Infidelity/01 - Don't Let Him Go.flac"),
		musicFile("/music/Hi Infidelity/02 - Keep On Loving You.flac"),
		musicFile("/music/Nine Lives/01 - Nine Lives.flac"),
		musicFile("/music/Nine Lives/02 - Falling in Love.flac"),
	}

	groups, err := g.Group(context.Background(), files)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}

	roots := map[string]int{}
	for _, gr := range groups {
		roots[gr.RootPath] = len(gr.Files)
	}
	if roots["/music/Hi Infidelity"] != 2 {
		t.Errorf("Hi Infidelity group has %d files, want 2", roots["/music/Hi Infidelity"])
	}
	if roots["/music/Nine Lives"] != 2 {
		t.Errorf("Nine Lives group has %d files, want 2", roots["/music/Nine Lives"])
	}
}

func TestMusicFolderGrouper_OnlyClaimsMusic(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	files := []domain.ScannedFile{
		musicFile("/music/Album/01.flac"),
		nonMusicFile("/video/Movie.mkv"),
		nonMusicFile("/video/Movie.nfo"),
	}

	groups, err := g.Group(context.Background(), files)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1 (only music file should be grouped)", len(groups))
	}
	for _, gr := range groups {
		for _, f := range gr.Files {
			if f.ContentType != domain.ContentTypeMusic {
				t.Errorf("non-music file %q ended up in a group", f.Path)
			}
		}
	}
}

func TestMusicFolderGrouper_EmptyInput(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	groups, err := g.Group(context.Background(), nil)
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("groups = %d, want 0 for empty input", len(groups))
	}
}

func TestMusicFolderGrouper_ContentTypes(t *testing.T) {
	g := identifier.NewMusicFolderGrouper()
	cts := g.ContentTypes()
	found := false
	for _, ct := range cts {
		if ct == domain.ContentTypeMusic {
			found = true
		}
	}
	if !found {
		t.Error("ContentTypes() does not include ContentTypeMusic")
	}
}

// Verify compile-time interface satisfaction.
var _ ports.FileGrouper = identifier.NewMusicFolderGrouper()
