package fs

import (
	"context"
	"fmt"
	"path/filepath"
	"purser/internal/ports"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacvorbis/v2"
	flac "github.com/go-flac/go-flac/v2"
)

const (
	vorbisAlbumIDKey = "MUSICBRAINZ_ALBUMID"
	vorbisTrackIDKey = "MUSICBRAINZ_TRACKID"
	id3AlbumIDDesc   = "MusicBrainz Album Id"
	id3TrackIDDesc   = "MusicBrainz Track Id"
)

type musicTagWriter struct{}

// NewMusicTagWriter returns a ports.MusicTagWriter that writes MusicBrainz
// release/recording IDs into FLAC (Vorbis comment) and MP3 (ID3v2 TXXX) tags,
// using the same field names MusicBrainz Picard writes so a later re-scan reads
// them back through the normal tag-reading path.
func NewMusicTagWriter() ports.MusicTagWriter {
	return &musicTagWriter{}
}

var _ ports.MusicTagWriter = (*musicTagWriter)(nil)

func (w *musicTagWriter) WriteIDs(_ context.Context, path, releaseMBID, recordingMBID string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		return writeFLACIDs(path, releaseMBID, recordingMBID)
	case ".mp3":
		return writeMP3IDs(path, releaseMBID, recordingMBID)
	default:
		return fmt.Errorf("write mbz ids %s: unsupported extension %s", path, filepath.Ext(path))
	}
}

func writeFLACIDs(path, releaseMBID, recordingMBID string) error {
	f, err := flac.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parse flac %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	cmt, idx := findVorbisComment(f)
	setVorbisComment(cmt, vorbisAlbumIDKey, releaseMBID)
	setVorbisComment(cmt, vorbisTrackIDKey, recordingMBID)

	block := cmt.Marshal()
	if idx >= 0 {
		f.Meta[idx] = &block
	} else {
		f.Meta = append(f.Meta, &block)
	}

	if err := f.Save(path); err != nil {
		return fmt.Errorf("save flac %s: %w", path, err)
	}
	return nil
}

func findVorbisComment(f *flac.File) (*flacvorbis.MetaDataBlockVorbisComment, int) {
	for i, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			if cmt, err := flacvorbis.ParseFromMetaDataBlock(*m); err == nil {
				return cmt, i
			}
		}
	}
	return flacvorbis.New(), -1
}

// setVorbisComment replaces every existing entry for key with a single entry
// carrying val. flacvorbis.Add always appends, so without this a stale entry
// from a prior write-back (or a re-scan) would accumulate duplicates.
func setVorbisComment(cmt *flacvorbis.MetaDataBlockVorbisComment, key, val string) {
	filtered := make([]string, 0, len(cmt.Comments)+1)
	for _, c := range cmt.Comments {
		parts := strings.SplitN(c, "=", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], key) {
			continue
		}
		filtered = append(filtered, c)
	}
	cmt.Comments = filtered
	_ = cmt.Add(key, val) // key is a package constant; never fails validation
}

func writeMP3IDs(path, releaseMBID, recordingMBID string) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("open mp3 %s: %w", path, err)
	}
	defer func() { _ = tag.Close() }()

	// AddUserDefinedTextFrame replaces any existing frame with the same
	// Description (id3v2's sequence dedupes on UniqueIdentifier), so re-writing
	// on a re-scan does not accumulate duplicate TXXX frames.
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
		Encoding:    id3v2.EncodingUTF8,
		Description: id3AlbumIDDesc,
		Value:       releaseMBID,
	})
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
		Encoding:    id3v2.EncodingUTF8,
		Description: id3TrackIDDesc,
		Value:       recordingMBID,
	})

	if err := tag.Save(); err != nil {
		return fmt.Errorf("save mp3 %s: %w", path, err)
	}
	return nil
}
