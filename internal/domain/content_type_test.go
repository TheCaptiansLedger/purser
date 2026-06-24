package domain

import "testing"

func TestContentType_Valid(t *testing.T) {
	valid := []ContentType{ContentTypeMovie, ContentTypeTV, ContentTypeMusic, ContentTypeAdult, ContentTypeJAV, ContentTypeBook}
	for _, ct := range valid {
		if !ct.Valid() {
			t.Errorf("ContentType(%q).Valid() = false, want true", ct)
		}
	}
	invalid := []ContentType{"", "unknown", "MOVIE"}
	for _, ct := range invalid {
		if ct.Valid() {
			t.Errorf("ContentType(%q).Valid() = true, want false", ct)
		}
	}
}

func TestKind_Valid(t *testing.T) {
	valid := []Kind{KindNetwork, KindStudio, KindSeries, KindArtist, KindAuthor, KindMovie, KindPublisher, KindBook}
	for _, k := range valid {
		if !k.Valid() {
			t.Errorf("Kind(%q).Valid() = false, want true", k)
		}
	}
	invalid := []Kind{"", "unknown", "STUDIO"}
	for _, k := range invalid {
		if k.Valid() {
			t.Errorf("Kind(%q).Valid() = true, want false", k)
		}
	}
}

func TestKind_SupportsMemberRelationships(t *testing.T) {
	if !KindArtist.SupportsMemberRelationships() {
		t.Error("KindArtist.SupportsMemberRelationships() = false, want true")
	}
	for _, k := range []Kind{KindNetwork, KindStudio, KindSeries, KindAuthor, KindMovie, KindPublisher, KindBook} {
		if k.SupportsMemberRelationships() {
			t.Errorf("Kind(%q).SupportsMemberRelationships() = true, want false", k)
		}
	}
}

func TestMonitorMode_Values(t *testing.T) {
	modes := []struct {
		mode MonitorMode
		want string
	}{
		{MonitorAll, "all"},
		{MonitorFuture, "future"},
		{MonitorNone, "none"},
		{MonitorLatest, "latest"},
	}
	for _, tc := range modes {
		if string(tc.mode) != tc.want {
			t.Errorf("MonitorMode value = %q, want %q", tc.mode, tc.want)
		}
	}
}
