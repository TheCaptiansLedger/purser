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
