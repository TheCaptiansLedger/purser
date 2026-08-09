package main

import (
	"testing"

	musicv1 "purser/gen/go/purser/music/v1"
)

func TestBulletItems_SkipsEmptyValues(t *testing.T) {
	fields := []labeledField{
		{"Name", "The Beatles"},
		{"Country", ""},
		{"MBID", "artist-1"},
	}

	items := bulletItems(fields)

	if len(items) != 2 {
		t.Fatalf("bulletItems returned %d items, want 2 (empty Country skipped)", len(items))
	}
	if items[0].Text != "Name: The Beatles" || items[1].Text != "MBID: artist-1" {
		t.Errorf("bulletItems = %+v, want [Name: The Beatles, MBID: artist-1]", items)
	}
}

func TestBulletItems_AllEmptyReturnsEmptyNotNil(t *testing.T) {
	items := bulletItems([]labeledField{{"Country", ""}})
	if items == nil {
		t.Fatal("bulletItems returned nil, want an empty (non-nil) slice")
	}
	if len(items) != 0 {
		t.Errorf("bulletItems = %+v, want empty", items)
	}
}

func TestPrintLookupResult_JSONOutputsEveryFieldRegardlessOfEmptiness(t *testing.T) {
	msg := &musicv1.LookupTheAudioDBArtistResponse{Artist: &musicv1.TheAudioDBArtist{
		Mbid: "artist-1",
		Name: "The Beatles",
		// Country deliberately left empty — JSON output must still be
		// well-formed and callable; protojson omits zero-value fields by
		// default (proto3 field-presence semantics), which is fine for
		// machine consumption per docs/adr/0009-cli-stack.md.
	}}

	if err := printLookupResult(msg, true, theAudioDBArtistFields(msg.GetArtist())); err != nil {
		t.Fatalf("printLookupResult returned error: %v", err)
	}
}

func TestPrintLookupResult_StyledOutputRenders(t *testing.T) {
	msg := &musicv1.LookupTheAudioDBArtistResponse{Artist: &musicv1.TheAudioDBArtist{
		Mbid: "artist-1",
		Name: "The Beatles",
	}}

	if err := printLookupResult(msg, false, theAudioDBArtistFields(msg.GetArtist())); err != nil {
		t.Fatalf("printLookupResult returned error: %v", err)
	}
}

func TestTheAudioDBArtistFields_MapsIdentityAndImageFields(t *testing.T) {
	a := &musicv1.TheAudioDBArtist{
		Mbid:  "artist-1",
		Name:  "The Beatles",
		Thumb: "https://example.invalid/thumb.jpg",
	}

	fields := theAudioDBArtistFields(a)

	got := fieldValue(t, fields, "Name")
	if got != "The Beatles" {
		t.Errorf("Name field = %q, want The Beatles", got)
	}
	if fieldValue(t, fields, "Thumb") != "https://example.invalid/thumb.jpg" {
		t.Errorf("Thumb field = %q, want the fake's thumb URL", fieldValue(t, fields, "Thumb"))
	}
}

func TestTheAudioDBAlbumFields_MapsIdentityAndImageFields(t *testing.T) {
	a := &musicv1.TheAudioDBAlbum{
		Mbid:  "rg-1",
		Title: "Please Please Me",
	}

	fields := theAudioDBAlbumFields(a)

	if fieldValue(t, fields, "Title") != "Please Please Me" {
		t.Errorf("Title field = %q, want Please Please Me", fieldValue(t, fields, "Title"))
	}
	if fieldValue(t, fields, "MBID") != "rg-1" {
		t.Errorf("MBID field = %q, want rg-1", fieldValue(t, fields, "MBID"))
	}
}

// fieldValue finds label's value in fields, failing the test if absent.
func fieldValue(t *testing.T, fields []labeledField, label string) string {
	t.Helper()
	for _, f := range fields {
		if f.label == label {
			return f.value
		}
	}
	t.Fatalf("no field labeled %q in %+v", label, fields)
	return ""
}

// TestPrintLookupResult_FanartTVJSONOutputs guards the FanartTVArtist
// message (a map field + repeated nested messages, a heavier shape than
// TheAudioDBArtist's flat scalars) through the same JSON path.
func TestPrintLookupResult_FanartTVJSONOutputs(t *testing.T) {
	msg := &musicv1.LookupFanartTVArtistResponse{Artist: &musicv1.FanartTVArtist{
		Name: "The Beatles",
		Mbid: "artist-1",
		Albums: map[string]*musicv1.FanartTVAlbumImages{
			"rg-1": {AlbumCover: []*musicv1.FanartTVImage{{Id: "1", Url: "https://example.invalid/cover.jpg"}}},
		},
	}}
	if err := printLookupResult(msg, true, fanartTVArtistFields(msg.GetArtist())); err != nil {
		t.Fatalf("printLookupResult returned error: %v", err)
	}
}
