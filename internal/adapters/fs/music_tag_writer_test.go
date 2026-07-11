package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis/v2"
	flac "github.com/go-flac/go-flac/v2"
)

func TestMusicTagWriter_WritesAndReadsBack_FLAC(t *testing.T) {
	dst := copyTestFLAC(t)

	before, _ := readVorbisComment(t, dst)
	artistBefore, err := before.Get("ARTIST")
	if err != nil {
		t.Fatalf("get artist before: %v", err)
	}

	w := NewMusicTagWriter()
	if err := w.WriteIDs(context.Background(), dst, "release-mbid-123", "recording-mbid-456"); err != nil {
		t.Fatalf("WriteIDs: %v", err)
	}

	after, _ := readVorbisComment(t, dst)

	albumIDs, err := after.Get(vorbisAlbumIDKey)
	if err != nil || len(albumIDs) != 1 || albumIDs[0] != "release-mbid-123" {
		t.Errorf("%s = %v, err %v, want [release-mbid-123]", vorbisAlbumIDKey, albumIDs, err)
	}
	trackIDs, err := after.Get(vorbisTrackIDKey)
	if err != nil || len(trackIDs) != 1 || trackIDs[0] != "recording-mbid-456" {
		t.Errorf("%s = %v, err %v, want [recording-mbid-456]", vorbisTrackIDKey, trackIDs, err)
	}

	artistAfter, err := after.Get("ARTIST")
	if err != nil {
		t.Fatalf("get artist after: %v", err)
	}
	if len(artistAfter) != len(artistBefore) || (len(artistAfter) > 0 && artistAfter[0] != artistBefore[0]) {
		t.Errorf("ARTIST tag changed: before=%v after=%v", artistBefore, artistAfter)
	}
}

func TestMusicTagWriter_FLAC_RewriteDoesNotDuplicate(t *testing.T) {
	dst := copyTestFLAC(t)

	w := NewMusicTagWriter()
	if err := w.WriteIDs(context.Background(), dst, "first-release", "first-recording"); err != nil {
		t.Fatalf("first WriteIDs: %v", err)
	}
	if err := w.WriteIDs(context.Background(), dst, "second-release", "second-recording"); err != nil {
		t.Fatalf("second WriteIDs: %v", err)
	}

	cmt, _ := readVorbisComment(t, dst)
	albumIDs, err := cmt.Get(vorbisAlbumIDKey)
	if err != nil {
		t.Fatalf("get album id: %v", err)
	}
	if len(albumIDs) != 1 || albumIDs[0] != "second-release" {
		t.Errorf("%s = %v, want exactly [second-release] — re-write must not accumulate duplicates", vorbisAlbumIDKey, albumIDs)
	}
}

func TestMusicTagWriter_WritesAndReadsBack_MP3(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "track.mp3")

	tag := id3v2.NewEmptyTag()
	tag.SetArtist("Test Artist")
	data, err := os.Create(dst) //nolint:gosec // test-only temp file
	if err != nil {
		t.Fatalf("create mp3 fixture: %v", err)
	}
	if _, err := tag.WriteTo(data); err != nil {
		t.Fatalf("write mp3 fixture tag: %v", err)
	}
	_ = data.Close()
	_ = tag.Close()

	w := NewMusicTagWriter()
	if err := w.WriteIDs(context.Background(), dst, "release-mbid-789", "recording-mbid-012"); err != nil {
		t.Fatalf("WriteIDs: %v", err)
	}

	got, err := id3v2.Open(dst, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-open mp3: %v", err)
	}
	defer func() { _ = got.Close() }()

	if got.Artist() != "Test Artist" {
		t.Errorf("Artist = %q, want %q", got.Artist(), "Test Artist")
	}

	var gotAlbumID, gotTrackID string
	for _, f := range got.GetFrames(got.CommonID("User defined text information frame")) {
		udtf, ok := f.(id3v2.UserDefinedTextFrame)
		if !ok {
			continue
		}
		switch udtf.Description {
		case id3AlbumIDDesc:
			gotAlbumID = udtf.Value
		case id3TrackIDDesc:
			gotTrackID = udtf.Value
		}
	}
	if gotAlbumID != "release-mbid-789" {
		t.Errorf("album id TXXX = %q, want %q", gotAlbumID, "release-mbid-789")
	}
	if gotTrackID != "recording-mbid-012" {
		t.Errorf("track id TXXX = %q, want %q", gotTrackID, "recording-mbid-012")
	}
}

func copyTestFLAC(t *testing.T) string {
	t.Helper()
	matches, err := filepath.Glob("../../../test-data/music/*/*.flac")
	if err != nil || len(matches) == 0 {
		t.Skip("no test FLAC fixtures found under test-data/music")
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read fixture %s: %v", matches[0], err)
	}
	dst := filepath.Join(t.TempDir(), "track.flac")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write copy %s: %v", dst, err)
	}
	return dst
}

func readVorbisComment(t *testing.T, path string) (*flacvorbis.MetaDataBlockVorbisComment, int) {
	t.Helper()
	f, err := flac.ParseFile(path)
	if err != nil {
		t.Fatalf("parse flac %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	return findVorbisComment(f)
}
