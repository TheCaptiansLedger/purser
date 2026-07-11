package identifier_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"purser/internal/adapters/fingerprint"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
	"time"
)

// ── FLAC binary helpers (mirrors pattern in fingerprint package's music_test.go) ─────────────

func tagsWriteUint24BE(buf *bytes.Buffer, n int) {
	buf.WriteByte(byte(n >> 16))
	buf.WriteByte(byte(n >> 8))
	buf.WriteByte(byte(n))
}

func tagsWriteLE32(buf *bytes.Buffer, n uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	buf.Write(b)
}

// tagsBuildFLAC builds a minimal valid FLAC file with the given VORBISCOMMENT tags.
func tagsBuildFLAC(comments map[string]string) []byte {
	var vc bytes.Buffer
	vendor := "purser test"
	tagsWriteLE32(&vc, uint32(len(vendor)))
	vc.WriteString(vendor)
	tagsWriteLE32(&vc, uint32(len(comments)))
	for k, v := range comments {
		c := k + "=" + v
		tagsWriteLE32(&vc, uint32(len(c)))
		vc.WriteString(c)
	}
	vcData := vc.Bytes()

	var buf bytes.Buffer
	buf.WriteString("fLaC")
	buf.WriteByte(0x00) // STREAMINFO block (type 0, not-last)
	tagsWriteUint24BE(&buf, 34)
	buf.Write(make([]byte, 34))
	buf.WriteByte(0x84) // VORBIS_COMMENT block (type 4, last)
	tagsWriteUint24BE(&buf, len(vcData))
	buf.Write(vcData)
	return buf.Bytes()
}

// fingerprintedFile builds a synthetic FLAC with the given VORBISCOMMENT tags, writes it
// to a temp file, fingerprints it with the real fingerprinter, and returns the ScannedFile
// with Fingerprint populated. No external tools or fixture files required.
func fingerprintedFile(t *testing.T, name string, comments map[string]string) domain.ScannedFile {
	t.Helper()
	data := tagsBuildFLAC(comments)
	f, err := os.CreateTemp(t.TempDir(), name+"*.flac")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	fp := fingerprint.NewMusicFingerprinter()
	sf := domain.ScannedFile{Path: f.Name(), ContentType: domain.ContentTypeMusic}
	result, err := fp.Fingerprint(context.Background(), sf)
	if err != nil {
		t.Fatalf("fingerprint %s: %v", name, err)
	}
	sf.Fingerprint = result
	return sf
}

// musicFileWithFP builds a ScannedFile with an in-memory Fingerprint from a tag map.
// Used for unit tests that do not need real file I/O.
func musicFileWithFP(path string, tags map[string]string) domain.ScannedFile {
	return domain.ScannedFile{
		Path:        path,
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{EmbeddedTags: tags},
	}
}

// ── unit tests ────────────────────────────────────────────────────────────────────────────────

func TestExtractMusicTagSummary_Consensus(t *testing.T) {
	group := ports.ScannedFileGroup{
		RootPath: "/music/Hi Infidelity",
		Files: []domain.ScannedFile{
			musicFileWithFP("/music/Hi Infidelity/01.flac", map[string]string{
				"album_artist": "REO Speedwagon",
				"album":        "Hi Infidelity",
				"barcode":      "0074646161425",
				"track_number": "1",
				"track_total":  "10",
				"disc_total":   "1",
				"title":        "Don't Let Him Go",
				"isrc":         "USSM10012807",
				"date":         "1981",
			}),
			musicFileWithFP("/music/Hi Infidelity/02.flac", map[string]string{
				"album_artist": "REO Speedwagon",
				"album":        "Hi Infidelity",
				"barcode":      "0074646161425",
				"track_number": "2",
				"track_total":  "10",
				"disc_total":   "1",
				"title":        "Keep on Loving You",
				"isrc":         "USSM10012808",
				"date":         "1981",
			}),
		},
	}

	summary := identifier.ExtractMusicTagSummary(context.Background(), group)

	if summary.AlbumArtist != "REO Speedwagon" {
		t.Errorf("AlbumArtist = %q, want %q", summary.AlbumArtist, "REO Speedwagon")
	}
	if summary.AlbumTitle != "Hi Infidelity" {
		t.Errorf("AlbumTitle = %q, want %q", summary.AlbumTitle, "Hi Infidelity")
	}
	if summary.Barcode != "0074646161425" {
		t.Errorf("Barcode = %q, want %q", summary.Barcode, "0074646161425")
	}
	if summary.Year != 1981 {
		t.Errorf("Year = %d, want 1981", summary.Year)
	}
	if summary.TotalTracks != 10 {
		t.Errorf("TotalTracks = %d, want 10", summary.TotalTracks)
	}
	if summary.TotalDiscs != 1 {
		t.Errorf("TotalDiscs = %d, want 1", summary.TotalDiscs)
	}
}

func TestExtractMusicTagSummary_AlbumArtist_FallsBackToArtistTag(t *testing.T) {
	group := ports.ScannedFileGroup{
		RootPath: "/music/Hi Infidelity",
		Files: []domain.ScannedFile{
			musicFileWithFP("/music/Hi Infidelity/01.flac", map[string]string{
				"artist":       "REO Speedwagon",
				"album":        "Hi Infidelity",
				"track_number": "1",
				"title":        "Don't Let Him Go",
			}),
			musicFileWithFP("/music/Hi Infidelity/02.flac", map[string]string{
				"artist":       "REO Speedwagon",
				"album":        "Hi Infidelity",
				"track_number": "2",
				"title":        "Keep on Loving You",
			}),
		},
	}

	summary := identifier.ExtractMusicTagSummary(context.Background(), group)

	if summary.AlbumArtist != "REO Speedwagon" {
		t.Errorf("AlbumArtist = %q, want %q (fallback from artist tag)", summary.AlbumArtist, "REO Speedwagon")
	}
}

func TestExtractMusicTagSummary_OutlierIgnored(t *testing.T) {
	files := make([]domain.ScannedFile, 10)
	for i := range files {
		barcode := "0074646161425"
		if i == 9 {
			barcode = "9999999999999"
		}
		files[i] = musicFileWithFP(fmt.Sprintf("/music/Album/%02d.flac", i+1), map[string]string{
			"album_artist": "REO Speedwagon",
			"barcode":      barcode,
			"disc_total":   "1",
			"track_number": fmt.Sprintf("%d", i+1),
		})
	}

	summary := identifier.ExtractMusicTagSummary(context.Background(), ports.ScannedFileGroup{
		RootPath: "/music/Album",
		Files:    files,
	})

	if summary.Barcode != "0074646161425" {
		t.Errorf("Barcode = %q, want majority value %q", summary.Barcode, "0074646161425")
	}
}

func TestExtractMusicTagSummary_TracksOrderedByNumber(t *testing.T) {
	group := ports.ScannedFileGroup{
		RootPath: "/music/Album",
		Files: []domain.ScannedFile{
			musicFileWithFP("/music/Album/03.flac", map[string]string{
				"track_number": "3", "disc_number": "1",
				"title": "Track Three", "isrc": "ISRC3", "duration_ms": "180000", "disc_total": "1",
			}),
			musicFileWithFP("/music/Album/01.flac", map[string]string{
				"track_number": "1", "disc_number": "1",
				"title": "Track One", "isrc": "ISRC1", "duration_ms": "120000", "disc_total": "1",
			}),
			musicFileWithFP("/music/Album/02.flac", map[string]string{
				"track_number": "2", "disc_number": "1",
				"title": "Track Two", "isrc": "ISRC2", "duration_ms": "150000", "disc_total": "1",
			}),
		},
	}

	summary := identifier.ExtractMusicTagSummary(context.Background(), group)

	wantTitles := []string{"Track One", "Track Two", "Track Three"}
	for i, want := range wantTitles {
		if summary.TrackTitles[i] != want {
			t.Errorf("TrackTitles[%d] = %q, want %q", i, summary.TrackTitles[i], want)
		}
	}
	wantISRCs := []string{"ISRC1", "ISRC2", "ISRC3"}
	for i, want := range wantISRCs {
		if summary.ISRCs[i] != want {
			t.Errorf("ISRCs[%d] = %q, want %q", i, summary.ISRCs[i], want)
		}
	}
	wantDurations := []time.Duration{120 * time.Second, 150 * time.Second, 180 * time.Second}
	for i, want := range wantDurations {
		if summary.TrackDurations[i] != want {
			t.Errorf("TrackDurations[%d] = %v, want %v", i, summary.TrackDurations[i], want)
		}
	}
}

func TestExtractMusicTagSummary_MultiDisc_DerivesTotalDiscs(t *testing.T) {
	// No disc_total tag — TotalDiscs must be derived from distinct disc_number values.
	group := ports.ScannedFileGroup{
		RootPath: "/music/The Wall",
		Files: []domain.ScannedFile{
			musicFileWithFP("/music/The Wall/CD1/01.flac", map[string]string{"disc_number": "1", "track_number": "1"}),
			musicFileWithFP("/music/The Wall/CD1/02.flac", map[string]string{"disc_number": "1", "track_number": "2"}),
			musicFileWithFP("/music/The Wall/CD2/01.flac", map[string]string{"disc_number": "2", "track_number": "1"}),
			musicFileWithFP("/music/The Wall/CD2/02.flac", map[string]string{"disc_number": "2", "track_number": "2"}),
		},
	}

	summary := identifier.ExtractMusicTagSummary(context.Background(), group)

	if summary.TotalDiscs != 2 {
		t.Errorf("TotalDiscs = %d, want 2 (derived from distinct disc_number values)", summary.TotalDiscs)
	}
}

func TestExtractMusicTagSummary_MBZReleaseIDPresent(t *testing.T) {
	const wantMBZID = "1e639bf3-6b4c-4e1a-9d15-c61511804c8f"
	group := ports.ScannedFileGroup{
		RootPath: "/music/Album",
		Files: []domain.ScannedFile{
			musicFileWithFP("", map[string]string{"musicbrainz_album_id": wantMBZID, "disc_total": "1"}),
			musicFileWithFP("", map[string]string{"musicbrainz_album_id": wantMBZID, "disc_total": "1"}),
		},
	}

	summary := identifier.ExtractMusicTagSummary(context.Background(), group)

	if summary.MBZReleaseID != wantMBZID {
		t.Errorf("MBZReleaseID = %q, want %q", summary.MBZReleaseID, wantMBZID)
	}
}

func TestExtractMusicTagSummary_NoConsensusEmitsWarn(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(old) })

	// 4 files: 2 say barcode "AAAA", 2 say "BBBB" — perfect tie, no consensus.
	group := ports.ScannedFileGroup{
		RootPath: "/music/Album",
		Files: []domain.ScannedFile{
			musicFileWithFP("", map[string]string{"barcode": "AAAA", "disc_total": "1"}),
			musicFileWithFP("", map[string]string{"barcode": "BBBB", "disc_total": "1"}),
			musicFileWithFP("", map[string]string{"barcode": "AAAA", "disc_total": "1"}),
			musicFileWithFP("", map[string]string{"barcode": "BBBB", "disc_total": "1"}),
		},
	}

	_ = identifier.ExtractMusicTagSummary(context.Background(), group)

	if !strings.Contains(buf.String(), "tag consensus failed") {
		t.Errorf("expected WARN 'tag consensus failed', got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "barcode") {
		t.Errorf("expected key=barcode in warn log, got: %s", buf.String())
	}
}

// ── integration tests (synthetic FLAC files, no on-disk fixtures required) ───────────────────

// TestExtractMusicTagSummary_Integration_REOSpeedwagon builds 10 synthetic FLAC files
// representing the REO Speedwagon "Hi Infidelity" single-disc album and asserts that
// ExtractMusicTagSummary correctly extracts barcode, track count, and all 10 ISRCs.
func TestExtractMusicTagSummary_Integration_REOSpeedwagon(t *testing.T) {
	tracks := []struct {
		num   int
		title string
		isrc  string
	}{
		{1, "Don't Let Him Go", "USSM10012807"},
		{2, "Keep on Loving You", "USSM10012808"},
		{3, "Follow My Heart", "USSM10012809"},
		{4, "In Your Letter", "USSM10012810"},
		{5, "Take It On the Run", "USSM10012811"},
		{6, "Tough Guys", "USSM10012812"},
		{7, "Out of Season", "USSM10012813"},
		{8, "Shakin' It Loose", "USSM10012814"},
		{9, "Someone Tonight", "USSM10012815"},
		{10, "I Wish You Were There", "USSM10012816"},
	}

	scanFiles := make([]domain.ScannedFile, 0, len(tracks))
	for _, tr := range tracks {
		sf := fingerprintedFile(t, fmt.Sprintf("reo_%02d", tr.num), map[string]string{
			"ALBUMARTIST":          "REO Speedwagon",
			"ALBUM":                "Hi Infidelity",
			"TITLE":                tr.title,
			"TRACKNUMBER":          fmt.Sprintf("%d", tr.num),
			"TRACKTOTAL":           "10",
			"DISCNUMBER":           "1",
			"DISCTOTAL":            "1",
			"DATE":                 "1981",
			"BARCODE":              "0074646161425",
			"ISRC":                 tr.isrc,
			"MUSICBRAINZ ALBUM ID": "1e639bf3-6b4c-4e1a-9d15-c61511804c8f",
			"LABEL":                "Epic - Legacy",
		})
		scanFiles = append(scanFiles, sf)
	}

	group := ports.ScannedFileGroup{RootPath: "/music/REO Speedwagon/Hi Infidelity", Files: scanFiles}
	summary := identifier.ExtractMusicTagSummary(context.Background(), group)
	t.Logf("MusicTagSummary: %+v", summary)

	if summary.Barcode != "0074646161425" {
		t.Errorf("Barcode = %q, want %q", summary.Barcode, "0074646161425")
	}
	if summary.TotalTracks != 10 {
		t.Errorf("TotalTracks = %d, want 10", summary.TotalTracks)
	}
	if summary.TotalDiscs != 1 {
		t.Errorf("TotalDiscs = %d, want 1", summary.TotalDiscs)
	}
	if summary.AlbumArtist != "REO Speedwagon" {
		t.Errorf("AlbumArtist = %q, want %q", summary.AlbumArtist, "REO Speedwagon")
	}
	if summary.MBZReleaseID != "1e639bf3-6b4c-4e1a-9d15-c61511804c8f" {
		t.Errorf("MBZReleaseID = %q", summary.MBZReleaseID)
	}
	if summary.Label != "Epic - Legacy" {
		t.Errorf("Label = %q, want %q", summary.Label, "Epic - Legacy")
	}
	if len(summary.ISRCs) != 10 {
		t.Fatalf("ISRCs count = %d, want 10", len(summary.ISRCs))
	}
	for i, isrc := range summary.ISRCs {
		if isrc == "" {
			t.Errorf("ISRCs[%d] is empty — all 10 must be non-empty", i)
		}
	}
}

// TestExtractMusicTagSummary_Integration_StevieNicks_ThreeDisc builds 12 synthetic FLAC
// files across 3 discs with no barcode, no ISRCs, and no DISCTOTAL tag, representing the
// worst-case fuzzy-pipeline input described in the implementation plan.
// Asserts TotalDiscs is derived from distinct DISCNUMBER values, tracks are ordered
// by (disc, track), and absent optional fields are correctly empty.
func TestExtractMusicTagSummary_Integration_StevieNicks_ThreeDisc(t *testing.T) {
	type trackSpec struct {
		disc  int
		track int
		title string
	}
	specs := []trackSpec{
		{1, 1, "Gold Dust Woman"},
		{1, 2, "Edge of Seventeen"},
		{1, 3, "Stand Back"},
		{1, 4, "Leather and Lace"},
		{2, 1, "Sara"},
		{2, 2, "Dreams"},
		{2, 3, "Rhiannon"},
		{2, 4, "Landslide"},
		{3, 1, "Gypsy"},
		{3, 2, "The Chain"},
		{3, 3, "Little Lies"},
		{3, 4, "Go Your Own Way"},
	}

	scanFiles := make([]domain.ScannedFile, 0, len(specs))
	for _, s := range specs {
		sf := fingerprintedFile(t, fmt.Sprintf("nicks_d%d_t%d", s.disc, s.track), map[string]string{
			"ALBUMARTIST": "Stevie Nicks",
			"ALBUM":       "The Enchanted Works of Stevie Nicks",
			"TITLE":       s.title,
			"TRACKNUMBER": fmt.Sprintf("%d", s.track),
			"TRACKTOTAL":  "4", // per-disc track count; no DISCTOTAL tag
			"DISCNUMBER":  fmt.Sprintf("%d", s.disc),
			"DATE":        "1991",
			// No BARCODE, no ISRC, no MUSICBRAINZ ALBUM ID — worst-case input
		})
		scanFiles = append(scanFiles, sf)
	}

	group := ports.ScannedFileGroup{
		RootPath: "/music/Stevie Nicks/The Enchanted Works of Stevie Nicks",
		Files:    scanFiles,
	}
	summary := identifier.ExtractMusicTagSummary(context.Background(), group)
	t.Logf("MusicTagSummary: %+v", summary)

	if summary.AlbumArtist != "Stevie Nicks" {
		t.Errorf("AlbumArtist = %q, want %q", summary.AlbumArtist, "Stevie Nicks")
	}
	if summary.AlbumTitle != "The Enchanted Works of Stevie Nicks" {
		t.Errorf("AlbumTitle = %q", summary.AlbumTitle)
	}
	if summary.TotalDiscs != 3 {
		t.Errorf("TotalDiscs = %d, want 3 (derived from disc_number fallback)", summary.TotalDiscs)
	}
	if summary.Barcode != "" {
		t.Errorf("Barcode = %q, want empty (no barcode in worst-case input)", summary.Barcode)
	}
	if len(summary.ISRCs) != 12 {
		t.Fatalf("ISRCs count = %d, want 12", len(summary.ISRCs))
	}
	for i, isrc := range summary.ISRCs {
		if isrc != "" {
			t.Errorf("ISRCs[%d] = %q, want empty (no ISRCs in worst-case input)", i, isrc)
		}
	}
	// Verify disc-then-track ordering: disc 1 track 1 must sort first.
	if len(summary.TrackTitles) > 0 && summary.TrackTitles[0] != "Gold Dust Woman" {
		t.Errorf("TrackTitles[0] = %q, want %q (disc 1 track 1 should sort first)", summary.TrackTitles[0], "Gold Dust Woman")
	}
	// Disc 2 track 1 must be at index 4.
	if len(summary.TrackTitles) > 4 && summary.TrackTitles[4] != "Sara" {
		t.Errorf("TrackTitles[4] = %q, want %q (disc 2 track 1)", summary.TrackTitles[4], "Sara")
	}
}
