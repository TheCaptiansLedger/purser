package fingerprint_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"purser/internal/adapters/fingerprint"
	"purser/internal/domain"
	"testing"
)

// --- ID3v2 binary builders ---

func syncsafeEncode(n int) []byte {
	b := make([]byte, 4)
	b[3] = byte(n & 0x7F)
	b[2] = byte((n >> 7) & 0x7F)
	b[1] = byte((n >> 14) & 0x7F)
	b[0] = byte((n >> 21) & 0x7F)
	return b
}

func buildID3v2Frame(id string, data []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString(id)
	sz := make([]byte, 4)
	binary.BigEndian.PutUint32(sz, uint32(len(data)))
	buf.Write(sz)
	buf.WriteByte(0x00) // flags byte 1
	buf.WriteByte(0x00) // flags byte 2
	buf.Write(data)
	return buf.Bytes()
}

// buildTextFrame builds an ID3v2 text frame (TIT2, TPE1, TALB, etc.) with Latin-1 encoding.
func buildTextFrame(id, value string) []byte {
	return buildID3v2Frame(id, append([]byte{0x00}, []byte(value)...))
}

// buildTXXXFrame builds an ID3v2 TXXX frame with Latin-1 encoding.
func buildTXXXFrame(description, value string) []byte {
	var data bytes.Buffer
	data.WriteByte(0x00) // Latin-1 encoding
	data.WriteString(description)
	data.WriteByte(0x00) // null separator between description and value
	data.WriteString(value)
	return buildID3v2Frame("TXXX", data.Bytes())
}

// buildID3v2MP3 wraps frame bytes in an ID3v2.3 header followed by a minimal MP3 sync frame.
func buildID3v2MP3(frames []byte) []byte {
	size := syncsafeEncode(len(frames))
	var buf bytes.Buffer
	buf.WriteString("ID3")
	buf.WriteByte(0x03) // ID3v2.3
	buf.WriteByte(0x00) // revision 0
	buf.WriteByte(0x00) // flags
	buf.Write(size)
	buf.Write(frames)
	// Minimal MPEG1 Layer3 128kbps 44.1kHz sync frame so the file is parseable as audio.
	buf.Write([]byte{0xFF, 0xFB, 0x90, 0x00})
	buf.Write(make([]byte, 413))
	return buf.Bytes()
}

// --- FLAC binary builders ---

func writeUint24BE(buf *bytes.Buffer, n int) {
	buf.WriteByte(byte(n >> 16))
	buf.WriteByte(byte(n >> 8))
	buf.WriteByte(byte(n))
}

func writeLE32(buf *bytes.Buffer, n uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	buf.Write(b)
}

// buildFLACWithVorbisComment constructs a minimal valid FLAC file containing the
// given VORBISCOMMENT key=value pairs. dhowden/tag lowercases all VORBISCOMMENT keys.
func buildFLACWithVorbisComment(comments map[string]string) []byte {
	var vc bytes.Buffer
	vendor := "purser test"
	writeLE32(&vc, uint32(len(vendor)))
	vc.WriteString(vendor)
	writeLE32(&vc, uint32(len(comments)))
	for k, v := range comments {
		c := k + "=" + v
		writeLE32(&vc, uint32(len(c)))
		vc.WriteString(c)
	}
	vcData := vc.Bytes()

	var buf bytes.Buffer
	buf.WriteString("fLaC")
	// STREAMINFO block (type 0, not-last): dhowden/tag seeks past it via Seek.
	buf.WriteByte(0x00)
	writeUint24BE(&buf, 34)
	buf.Write(make([]byte, 34))
	// VORBIS_COMMENT block (type 4, last).
	buf.WriteByte(0x84)
	writeUint24BE(&buf, len(vcData))
	buf.Write(vcData)
	return buf.Bytes()
}

// --- Tests ---

func TestMusicFingerprinter_ContentTypes(t *testing.T) {
	fp := fingerprint.NewMusicFingerprinter()
	cts := fp.ContentTypes()
	if len(cts) != 1 || cts[0] != domain.ContentTypeMusic {
		t.Errorf("ContentTypes() = %v, want [music]", cts)
	}
}

func TestMusicFingerprinter_UnsupportedType(t *testing.T) {
	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        "/fake/file.mkv",
		ContentType: domain.ContentTypeMovie,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil for non-music content type, got %+v", result)
	}
}

func TestMusicFingerprinter_ID3v2Tags(t *testing.T) {
	var frames bytes.Buffer
	frames.Write(buildTextFrame("TIT2", "Test Title"))
	frames.Write(buildTextFrame("TPE1", "Test Artist"))
	frames.Write(buildTextFrame("TALB", "Test Album"))

	path := writeTempFile(t, ".mp3", buildID3v2MP3(frames.Bytes()))

	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}
	if result.EmbeddedTags["title"] != "Test Title" {
		t.Errorf("title = %q, want Test Title", result.EmbeddedTags["title"])
	}
	if result.EmbeddedTags["artist"] != "Test Artist" {
		t.Errorf("artist = %q, want Test Artist", result.EmbeddedTags["artist"])
	}
	if result.EmbeddedTags["album"] != "Test Album" {
		t.Errorf("album = %q, want Test Album", result.EmbeddedTags["album"])
	}
}

func TestMusicFingerprinter_MusicBrainzID3v2(t *testing.T) {
	const wantID = "550e8400-e29b-41d4-a716-446655440000"

	var frames bytes.Buffer
	frames.Write(buildTXXXFrame("MusicBrainz Track Id", wantID))

	path := writeTempFile(t, ".mp3", buildID3v2MP3(frames.Bytes()))

	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}
	if result.EmbeddedTags["musicbrainz_track_id"] != wantID {
		t.Errorf("musicbrainz_track_id = %q, want %q", result.EmbeddedTags["musicbrainz_track_id"], wantID)
	}
}

func TestApplyFpcalcOutput_PopulatesAcoustIDAndDuration(t *testing.T) {
	fp := &domain.Fingerprint{EmbeddedTags: map[string]string{}}
	// Non-raw fpcalc output: FINGERPRINT is the base64url Chromaprint string.
	output := "FILE=/music/test.flac\nDURATION=252.86\nFINGERPRINT=AQADtNSmiUmScEiS\n"
	if err := fingerprint.ApplyFpcalcOutput(output, fp); err != nil {
		t.Fatal(err)
	}
	if fp.AcoustID != "AQADtNSmiUmScEiS" {
		t.Errorf("AcoustID = %q, want AQADtNSmiUmScEiS", fp.AcoustID)
	}
	if fp.EmbeddedTags["duration_ms"] != "252860" {
		t.Errorf("duration_ms = %q, want 252860", fp.EmbeddedTags["duration_ms"])
	}
}

func TestApplyFpcalcOutput_MissingFingerprint(t *testing.T) {
	fp := &domain.Fingerprint{EmbeddedTags: map[string]string{}}
	if err := fingerprint.ApplyFpcalcOutput("DURATION=252.86\n", fp); err == nil {
		t.Error("expected error for missing FINGERPRINT, got nil")
	}
}

func TestApplyFpcalcOutput_ZeroDurationIgnored(t *testing.T) {
	fp := &domain.Fingerprint{EmbeddedTags: map[string]string{}}
	_ = fingerprint.ApplyFpcalcOutput("DURATION=0\nFINGERPRINT=abc\n", fp)
	if _, ok := fp.EmbeddedTags["duration_ms"]; ok {
		t.Error("duration_ms should not be set for zero duration")
	}
}

func TestMusicFingerprinter_TagTable(t *testing.T) {
	const (
		wantISRC      = "TSTEST00001"
		wantBarcode   = "012345678901"
		wantLabel     = "Test Label"
		wantCatalog   = "CAT-001"
		wantRGID      = "aaaabbbb-cccc-dddd-eeee-ffffffffffff"
		wantAArtistID = "11112222-3333-4444-5555-666677778888"
	)

	tests := []struct {
		name      string
		buildFile func(t *testing.T) string
		wantKey   string
		wantVal   string
	}{
		// VORBISCOMMENT (FLAC) — new extraction keys
		{
			name: "FLAC ISRC",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"ISRC": wantISRC}))
			},
			wantKey: "isrc", wantVal: wantISRC,
		},
		{
			name: "FLAC UPC maps to barcode",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"UPC": wantBarcode}))
			},
			wantKey: "barcode", wantVal: wantBarcode,
		},
		{
			name: "FLAC BARCODE",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"BARCODE": wantBarcode}))
			},
			wantKey: "barcode", wantVal: wantBarcode,
		},
		{
			name: "FLAC LABEL",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"LABEL": wantLabel}))
			},
			wantKey: "label", wantVal: wantLabel,
		},
		{
			name: "FLAC CATALOGNUMBER",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"CATALOGNUMBER": wantCatalog}))
			},
			wantKey: "catalog_number", wantVal: wantCatalog,
		},
		{
			name: "FLAC TRACKTOTAL",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"TRACKTOTAL": "12"}))
			},
			wantKey: "track_total", wantVal: "12",
		},
		{
			name: "FLAC TOTALTRACKS",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"TOTALTRACKS": "8"}))
			},
			wantKey: "track_total", wantVal: "8",
		},
		{
			name: "FLAC DISCTOTAL",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"DISCTOTAL": "2"}))
			},
			wantKey: "disc_total", wantVal: "2",
		},
		{
			name: "FLAC TOTALDISCS",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"TOTALDISCS": "3"}))
			},
			wantKey: "disc_total", wantVal: "3",
		},
		{
			name: "FLAC MUSICBRAINZ_RELEASEGROUPID",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"MUSICBRAINZ_RELEASEGROUPID": wantRGID}))
			},
			wantKey: "musicbrainz_release_group_id", wantVal: wantRGID,
		},
		{
			name: "FLAC MUSICBRAINZ_ALBUMARTISTID",
			buildFile: func(t *testing.T) string {
				return writeTempFile(t, ".flac", buildFLACWithVorbisComment(map[string]string{"MUSICBRAINZ_ALBUMARTISTID": wantAArtistID}))
			},
			wantKey: "musicbrainz_album_artist_id", wantVal: wantAArtistID,
		},
		// ID3v2 standard frames
		{
			name: "ID3v2 TSRC → isrc",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTextFrame("TSRC", wantISRC))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "isrc", wantVal: wantISRC,
		},
		{
			name: "ID3v2 TPUB → label",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTextFrame("TPUB", wantLabel))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "label", wantVal: wantLabel,
		},
		{
			name: "ID3v2 TRCK N/total → track_total",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTextFrame("TRCK", "3/10"))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "track_total", wantVal: "10",
		},
		{
			name: "ID3v2 TPOS N/total → disc_total",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTextFrame("TPOS", "1/2"))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "disc_total", wantVal: "2",
		},
		// ID3v2 TXXX frames
		{
			name: "ID3v2 TXXX BARCODE → barcode",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTXXXFrame("BARCODE", wantBarcode))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "barcode", wantVal: wantBarcode,
		},
		{
			name: "ID3v2 TXXX MusicBrainz Release Group Id",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTXXXFrame("MusicBrainz Release Group Id", wantRGID))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "musicbrainz_release_group_id", wantVal: wantRGID,
		},
		{
			name: "ID3v2 TXXX MusicBrainz Album Artist Id",
			buildFile: func(t *testing.T) string {
				var f bytes.Buffer
				f.Write(buildTXXXFrame("MusicBrainz Album Artist Id", wantAArtistID))
				return writeTempFile(t, ".mp3", buildID3v2MP3(f.Bytes()))
			},
			wantKey: "musicbrainz_album_artist_id", wantVal: wantAArtistID,
		},
	}

	fp := fingerprint.NewMusicFingerprinter()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.buildFile(t)
			result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
				Path:        path,
				ContentType: domain.ContentTypeMusic,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result == nil {
				t.Fatal("expected non-nil Fingerprint")
			}
			if got := result.EmbeddedTags[tc.wantKey]; got != tc.wantVal {
				t.Errorf("EmbeddedTags[%q] = %q, want %q", tc.wantKey, got, tc.wantVal)
			}
		})
	}
}

func TestMusicFingerprinter_FLACVorbisComment(t *testing.T) {
	const wantTitle = "FLAC Test Track"
	const wantArtist = "FLAC Test Artist"
	const wantMBID = "660e9500-e29b-41d4-a716-556655440000"

	data := buildFLACWithVorbisComment(map[string]string{
		"TITLE":               wantTitle,
		"ARTIST":              wantArtist,
		"MUSICBRAINZ_TRACKID": wantMBID,
	})
	path := writeTempFile(t, ".flac", data)

	fp := fingerprint.NewMusicFingerprinter()
	result, err := fp.Fingerprint(context.Background(), domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil Fingerprint")
	}
	if result.EmbeddedTags["title"] != wantTitle {
		t.Errorf("title = %q, want %q", result.EmbeddedTags["title"], wantTitle)
	}
	if result.EmbeddedTags["artist"] != wantArtist {
		t.Errorf("artist = %q, want %q", result.EmbeddedTags["artist"], wantArtist)
	}
	if result.EmbeddedTags["musicbrainz_track_id"] != wantMBID {
		t.Errorf("musicbrainz_track_id = %q, want %q", result.EmbeddedTags["musicbrainz_track_id"], wantMBID)
	}
}
