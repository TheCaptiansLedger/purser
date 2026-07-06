package domain

import "testing"

func TestMusicRelease_ZeroValueSurvivesApplyDefaults(t *testing.T) {
	var r MusicRelease
	r.ApplyDefaults()
	if r.AddedAt.IsZero() {
		t.Error("AddedAt should be set after ApplyDefaults")
	}
	if r.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after ApplyDefaults")
	}
	if r.Status != ReleaseStatusStub {
		t.Errorf("Status = %q, want %q", r.Status, ReleaseStatusStub)
	}
}

func TestReleaseStatus_DistinctConstants(t *testing.T) {
	statuses := []ReleaseStatus{ReleaseStatusStub, ReleaseStatusPartial, ReleaseStatusImported}
	seen := make(map[ReleaseStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate ReleaseStatus value: %q", s)
		}
		seen[s] = true
	}
}

func TestMusicConfidenceSignals_AllFieldsZero(t *testing.T) {
	var s MusicConfidenceSignals
	if s.Barcode != 0 || s.ISRC != 0 || s.RGNameFuzzy != 0 ||
		s.TrackCount != 0 || s.TrackTitleSet != 0 || s.Duration != 0 || s.AcoustID != 0 {
		t.Error("zero-value MusicConfidenceSignals must have all fields at 0.0")
	}
}

func TestMusicScanGroup_ReusesUnmatchedStatus(t *testing.T) {
	g := MusicScanGroup{Status: UnmatchedPending}
	if g.Status != UnmatchedPending {
		t.Errorf("Status = %q, want %q", g.Status, UnmatchedPending)
	}
	g.Status = UnmatchedMatched
	if g.Status != UnmatchedMatched {
		t.Errorf("Status = %q, want %q", g.Status, UnmatchedMatched)
	}
	g.Status = UnmatchedDismissed
	if g.Status != UnmatchedDismissed {
		t.Errorf("Status = %q, want %q", g.Status, UnmatchedDismissed)
	}
}

func TestMediaFile_SHA1RoundTrip(t *testing.T) {
	const hash = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	mf := MediaFile{SHA1: hash}
	if mf.SHA1 != hash {
		t.Errorf("SHA1 = %q, want %q", mf.SHA1, hash)
	}
	var zero MediaFile
	if zero.SHA1 != "" {
		t.Errorf("zero-value SHA1 = %q, want empty", zero.SHA1)
	}
}

func TestMediaFile_MetadataRoundTrip(t *testing.T) {
	mf := MediaFile{Metadata: map[string]string{"acoustid": "abc123"}}
	if got := mf.Metadata["acoustid"]; got != "abc123" {
		t.Errorf("Metadata[acoustid] = %q, want %q", got, "abc123")
	}
	var zero MediaFile
	if zero.Metadata != nil {
		t.Error("zero-value Metadata should be nil")
	}
}
