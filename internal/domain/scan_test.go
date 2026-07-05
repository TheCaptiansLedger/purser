package domain

import (
	"encoding/json"
	"testing"
)

func TestMusicMatchDetail(t *testing.T) {
	t.Run("full round-trip", func(t *testing.T) {
		original := MusicMatchDetail{
			RecordingMBID:       "rec-mbid-123",
			RecordingTitle:      "Take It on the Run",
			RecordingConfidence: 0.97,
			ReleaseGroupMBID:    "rg-mbid-456",
			ReleaseMBID:         "rel-mbid-789",
			ReleaseTitle:        "Hi Infidelity",
			ReleaseDate:         "1980-11-01",
			ReleaseLabel:        "Epic",
			ReleaseCountry:      "US",
			ReleaseCatalog:      "FE 36844",
			ReleaseBarcode:      "07464368442",
			ReleaseConfidence:   0.82,
			MatchReasons: MatchReasons{
				Fingerprint:  0.95,
				Duration:     1.0,
				TitleTag:     1.0,
				ArtistTag:    1.0,
				AlbumTag:     1.0,
				AlbumContext: 0.10,
			},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got MusicMatchDetail
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got != original {
			t.Errorf("round-trip mismatch\ngot:  %+v\nwant: %+v", got, original)
		}
	})

	t.Run("omitempty: zero strings absent from JSON", func(t *testing.T) {
		d := MusicMatchDetail{RecordingConfidence: 0.50, ReleaseConfidence: 0.50}
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal to map: %v", err)
		}
		for _, field := range []string{
			"recording_mbid", "recording_title", "release_group_mbid",
			"release_mbid", "release_title", "release_date", "release_label",
			"release_country", "release_catalog", "release_barcode",
		} {
			if _, ok := m[field]; ok {
				t.Errorf("field %q present in JSON but should be omitted when empty", field)
			}
		}
	})

	t.Run("zero-value safe", func(_ *testing.T) {
		var d MusicMatchDetail
		_ = d.RecordingMBID
		_ = d.RecordingConfidence
		_ = d.ReleaseGroupMBID
		_ = d.ReleaseMBID
		_ = d.ReleaseConfidence
		_ = d.MatchReasons.Fingerprint
		_ = d.MatchReasons.Duration
		_ = d.MatchReasons.TitleTag
		_ = d.MatchReasons.ArtistTag
		_ = d.MatchReasons.AlbumTag
		_ = d.MatchReasons.AlbumContext
	})
}

func TestMatchCandidate(t *testing.T) {
	t.Run("new fields zero-valued by default", func(t *testing.T) {
		c := MatchCandidate{Confidence: 0.87}
		if c.RecordingConfidence != 0 {
			t.Errorf("RecordingConfidence = %v, want 0", c.RecordingConfidence)
		}
		if c.ReleaseConfidence != 0 {
			t.Errorf("ReleaseConfidence = %v, want 0", c.ReleaseConfidence)
		}
		if c.MusicDetail != nil {
			t.Error("MusicDetail should be nil by default")
		}
	})

	t.Run("music detail accessible via pointer", func(t *testing.T) {
		c := MatchCandidate{
			Confidence:          0.92,
			RecordingConfidence: 0.97,
			ReleaseConfidence:   0.82,
			Source:              string(MatchSourceAcoustID),
			MusicDetail: &MusicMatchDetail{
				RecordingMBID:    "rec-mbid-123",
				ReleaseGroupMBID: "rg-mbid-456",
			},
		}
		if c.MusicDetail.RecordingMBID != "rec-mbid-123" {
			t.Errorf("MusicDetail.RecordingMBID = %q, want %q", c.MusicDetail.RecordingMBID, "rec-mbid-123")
		}
		if c.MusicDetail.ReleaseGroupMBID != "rg-mbid-456" {
			t.Errorf("MusicDetail.ReleaseGroupMBID = %q, want %q", c.MusicDetail.ReleaseGroupMBID, "rg-mbid-456")
		}
	})
}

func TestMediaFile_MatchDetailNil(t *testing.T) {
	mf := MediaFile{ID: "test-id"}
	if mf.MatchDetail != nil {
		t.Error("MatchDetail should be nil by default")
	}
	data, err := json.Marshal(mf.MatchDetail)
	if err != nil {
		t.Fatalf("marshal nil MatchDetail: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("nil MatchDetail marshals as %q, want %q", string(data), "null")
	}
}
