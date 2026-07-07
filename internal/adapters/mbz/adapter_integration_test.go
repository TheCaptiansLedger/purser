//go:build integration

package mbz_test

import (
	"context"
	"errors"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
	"time"
)

// beatlesMBID is The Beatles' canonical MusicBrainz artist identifier.
// Stable since MBZ inception; used as an anchor for all integration tests.
const beatlesMBID = "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d"

// reoSpeedwagonMBID is REO Speedwagon's canonical MusicBrainz identifier.
// Used as the anchor for artist enrichment tests — their metadata is stable
// and all enrichment fields (aliases, ISNI, URL relations) are well-populated.
const reoSpeedwagonMBID = "bdc70372-7e8a-4cb9-8d33-f036b3b7cdc1"

func newIntegrationAdapter() *mbz.Adapter {
	return mbz.New(config.MetadataSourceConfig{}, nil)
}

func integrationCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// TestMBZAdapter_Integration_REOSpeedwagon_Enrichment verifies that
// FetchEntryMetadata populates all enrichment keys for a known group artist.
func TestMBZAdapter_Integration_REOSpeedwagon_Enrichment(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	meta, err := a.FetchEntryMetadata(ctx, domain.ContentTypeMusic, reoSpeedwagonMBID)
	if err != nil {
		t.Fatalf("FetchEntryMetadata: %v", err)
	}

	t.Logf("enrichment metadata: %v", meta)

	if meta["artist_type"] != "group" {
		t.Errorf("artist_type = %q, want group", meta["artist_type"])
	}
	if v, _ := meta["founded_date"].(string); v == "" {
		t.Errorf("founded_date is empty, want non-empty string")
	}
	if v, _ := meta["founded_location"].(string); v == "" {
		t.Errorf("founded_location is empty, want non-empty string")
	}
	if _, ok := meta["aliases"]; !ok {
		t.Error("aliases key must be present")
	}
	if v, _ := meta["official_url"].(string); v == "" {
		t.Errorf("official_url is empty, want non-empty string")
	}
	if v, _ := meta["lastfm_url"].(string); v == "" {
		t.Errorf("lastfm_url is empty, want non-empty string")
	}
	if _, ok := meta["born_date"]; ok {
		t.Error("born_date must not be set for a group artist")
	}
}

// TestMBZ_SearchStudios verifies the artist search returns both groups and solo
// artists. Searching "The Beatles" must surface the canonical entry.
func TestMBZ_SearchStudios(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchStudios(ctx, "The Beatles", 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchStudios returned no results for 'The Beatles'")
	}

	// First result must be the canonical Beatles entry.
	got := results[0]
	if got.ExternalID != beatlesMBID {
		t.Errorf("results[0].ExternalID = %q, want %q", got.ExternalID, beatlesMBID)
	}
	if got.Source != domain.SourceMusicBrainz {
		t.Errorf("results[0].Source = %q, want %q", got.Source, domain.SourceMusicBrainz)
	}
	if got.Name == "" {
		t.Error("results[0].Name is empty")
	}
}

// TestMBZ_SearchStudios_SoloArtist verifies that the absence of a type:Group
// filter means solo artists (Person type in MBZ) are also returned.
func TestMBZ_SearchStudios_SoloArtist(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchStudios(ctx, "Stevie Nicks", 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchStudios returned no results for 'Stevie Nicks' — solo artists must not be filtered out")
	}

	found := false
	for _, r := range results {
		if r.Name == "Stevie Nicks" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'Stevie Nicks' not found in results — solo artist (Person type) must be returned by SearchStudios")
	}
}

// TestMBZ_SearchPeople verifies the person search returns individual artists.
func TestMBZ_SearchPeople(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchPeople(ctx, "John Lennon", 10)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchPeople returned no results for 'John Lennon'")
	}

	got := results[0]
	if got.Source != domain.SourceMusicBrainz {
		t.Errorf("results[0].Source = %q, want %q", got.Source, domain.SourceMusicBrainz)
	}
	if got.ExternalID == "" {
		t.Error("results[0].ExternalID is empty")
	}
	if got.Name == "" {
		t.Error("results[0].Name is empty")
	}
	if got.Role != domain.RoleArtist {
		t.Errorf("results[0].Role = %q, want %q", got.Role, domain.RoleArtist)
	}
}

// TestMBZ_SearchItems verifies the recording search returns tracks.
func TestMBZ_SearchItems(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchItems(ctx, domain.ContentTypeMusic, "Come Together", 10)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchItems returned no results for 'Come Together'")
	}

	got := results[0]
	if got.Source != domain.SourceMusicBrainz {
		t.Errorf("results[0].Source = %q, want %q", got.Source, domain.SourceMusicBrainz)
	}
	if got.ExternalID == "" {
		t.Error("results[0].ExternalID is empty")
	}
	if got.Title == "" {
		t.Error("results[0].Title is empty")
	}
}

// TestMBZ_FindByExternalID verifies artist lookup by MBID returns correct data.
func TestMBZ_FindByExternalID(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	item, err := a.FindByExternalID(ctx, domain.ContentTypeMusic, beatlesMBID)
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}
	if item.ExternalID != beatlesMBID {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, beatlesMBID)
	}
	if item.Title != "The Beatles" {
		t.Errorf("Title = %q, want %q", item.Title, "The Beatles")
	}
	if item.Source != domain.SourceMusicBrainz {
		t.Errorf("Source = %q, want %q", item.Source, domain.SourceMusicBrainz)
	}
}

// TestMBZ_FindByExternalID_NotFound verifies that a nonexistent MBID returns
// ErrNotFound rather than a generic error.
func TestMBZ_FindByExternalID_NotFound(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	_, err := a.FindByExternalID(ctx, domain.ContentTypeMusic, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ports.ErrNotFound for nonexistent MBID, got: %v", err)
	}
}

// TestMBZ_FindByHash_NotSupported confirms MBZ does not support hash lookup.
func TestMBZ_FindByHash_NotSupported(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	_, err := a.FindByHash(ctx, "abc123")
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ports.ErrNotSupported, got: %v", err)
	}
}

// TestMBZ_FetchEntryContent verifies that browsing an artist's release-groups
// returns paginated groups with years parsed from first-release-date.
func TestMBZ_FetchEntryContent(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	groups, items, total, err := a.FetchEntryContent(ctx, domain.ContentTypeMusic, beatlesMBID, 1, 10)
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if items != nil {
		t.Error("items should be nil for music hierarchy (tracks are fetched via FetchGroupContent)")
	}
	if total == 0 {
		t.Fatal("total = 0; The Beatles must have release-groups in MusicBrainz")
	}
	if len(groups) == 0 {
		t.Fatal("groups is empty; expected at least one release-group on page 1")
	}

	for i, g := range groups {
		if g.ExternalID == "" {
			t.Errorf("groups[%d].ExternalID is empty", i)
		}
		if g.Title == "" {
			t.Errorf("groups[%d].Title is empty", i)
		}
		if g.Source != domain.SourceMusicBrainz {
			t.Errorf("groups[%d].Source = %q, want %q", i, g.Source, domain.SourceMusicBrainz)
		}
		// Year comes from first-release-date. Legitimately 0 if MBZ has no date, so just validate range.
		if g.Year != 0 && (g.Year < 1900 || g.Year > 2100) {
			t.Errorf("groups[%d].Year = %d, outside plausible range", i, g.Year)
		}
		if g.PrimaryType == "" {
			t.Errorf("groups[%d].PrimaryType is empty; Official releases always carry a primary type", i)
		}
	}
}

// TestMBZ_FetchEntryPeople verifies that band members are returned for a group
// artist. The Beatles are used; Ringo Starr is asserted as a known member.
func TestMBZ_FetchEntryPeople(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	members, err := a.FetchEntryPeople(ctx, beatlesMBID)
	if err != nil {
		t.Fatalf("FetchEntryPeople: %v", err)
	}
	if len(members) == 0 {
		t.Fatal("FetchEntryPeople returned no members for The Beatles")
	}

	found := false
	for _, m := range members {
		if m.ExternalID == "" {
			t.Errorf("member %q has empty ExternalID", m.Name)
		}
		if m.Name == "" {
			t.Errorf("member with ExternalID %q has empty Name", m.ExternalID)
		}
		if m.Source != domain.SourceMusicBrainz {
			t.Errorf("member %q Source = %q, want %q", m.Name, m.Source, domain.SourceMusicBrainz)
		}
		if m.Role != domain.RoleArtist {
			t.Errorf("member %q Role = %q, want %q", m.Name, m.Role, domain.RoleArtist)
		}
		if m.Name == "Ringo Starr" {
			found = true
		}
	}
	if !found {
		t.Error("Ringo Starr not found in Beatles members")
	}
}

// TestIntegrationRecordingGroupMBID verifies that FetchRecordingByID returns a
// release group MBID in GroupExternalID, not a release MBID. It obtains a
// recording MBID dynamically from The Beatles discography to avoid hardcoding
// brittle IDs. The critical invariant: GroupExternalID != "" and
// GroupExternalID != item.ExternalID (recording MBID).
func TestIntegrationRecordingGroupMBID(t *testing.T) {
	a := newIntegrationAdapter()

	// Step 1 — get a release-group from The Beatles discography.
	ctx1, cancel1 := integrationCtx(t)
	defer cancel1()

	groups, _, _, err := a.FetchEntryContent(ctx1, domain.ContentTypeMusic, beatlesMBID, 1, 5)
	if err != nil {
		t.Fatalf("FetchEntryContent (setup): %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("FetchEntryContent returned no groups; cannot run recording group MBID test")
	}

	// Step 2 — get a recording MBID from that release-group.
	ctx2, cancel2 := integrationCtx(t)
	defer cancel2()

	items, _, err := a.FetchGroupContent(ctx2, domain.ContentTypeMusic, groups[0].ExternalID, 1, 5)
	if err != nil {
		t.Fatalf("FetchGroupContent (setup): %v", err)
	}
	if len(items) == 0 {
		t.Fatal("FetchGroupContent returned no items; cannot run recording group MBID test")
	}
	recordingMBID := items[0].ExternalID

	// Step 3 — fetch the recording by ID and assert GroupExternalID is a release group MBID.
	ctx3, cancel3 := integrationCtx(t)
	defer cancel3()

	item, err := a.FetchRecordingByID(ctx3, recordingMBID, "")
	if err != nil {
		t.Fatalf("FetchRecordingByID(%q): %v", recordingMBID, err)
	}
	if item.ExternalID != recordingMBID {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, recordingMBID)
	}
	if item.GroupExternalID == "" {
		t.Error("GroupExternalID is empty; MBZ must return a release-group for any canonical recording")
	}
	// Core invariant: recording MBID and release group MBID are different MBZ entity types.
	if item.GroupExternalID == item.ExternalID {
		t.Errorf("GroupExternalID == ExternalID (%q); release group MBID must differ from recording MBID", item.ExternalID)
	}
	// ReleaseDetail carries the selected release MBID; it must also differ from the release group MBID.
	if item.ReleaseDetail != nil && item.ReleaseDetail.ReleaseMBID == item.GroupExternalID {
		t.Errorf("ReleaseDetail.ReleaseMBID (%q) == GroupExternalID (%q); release and release group are different MBZ entities",
			item.ReleaseDetail.ReleaseMBID, item.GroupExternalID)
	}
}

// TestMBZAdapter_Integration_HiInfidelity_AllReleases verifies that
// FetchReleaseGroupReleases returns all known pressings for REO Speedwagon's
// "Hi Infidelity" release group: ≥5 releases, the 2024 Digital edition with
// barcode 074646161425 must be present, and exactly one release is IsDefault.
func TestMBZAdapter_Integration_HiInfidelity_AllReleases(t *testing.T) {
	a := newIntegrationAdapter()

	// Step 1 — locate the Hi Infidelity release group in REO Speedwagon's discography.
	var hiInfMBID string
	for page := 1; page <= 5 && hiInfMBID == ""; page++ {
		ctx, cancel := integrationCtx(t)
		groups, _, _, err := a.FetchEntryContent(ctx, domain.ContentTypeMusic, reoSpeedwagonMBID, page, 20)
		cancel()
		if err != nil {
			t.Fatalf("FetchEntryContent (page %d): %v", page, err)
		}
		for _, g := range groups {
			if containsFold(g.Title, "hi infidelity") {
				hiInfMBID = g.ExternalID
				break
			}
		}
		if len(groups) == 0 {
			break
		}
	}
	if hiInfMBID == "" {
		t.Skip("Hi Infidelity release group not found in REO Speedwagon discography")
	}

	// Step 2 — fetch all known releases for Hi Infidelity.
	ctx, cancel := integrationCtx(t)
	defer cancel()

	releases, err := a.FetchReleaseGroupReleases(ctx, hiInfMBID)
	if err != nil {
		t.Fatalf("FetchReleaseGroupReleases: %v", err)
	}

	t.Logf("Hi Infidelity: %d releases (rg=%s)", len(releases), hiInfMBID)
	for _, r := range releases {
		t.Logf("  %s %q country=%s date=%s barcode=%q is_default=%v",
			r.MBID, r.Title, r.Country, r.Date, r.Barcode, r.IsDefault)
	}

	if len(releases) < 5 {
		t.Errorf("expected ≥5 releases, got %d", len(releases))
	}

	hasExpectedBarcode := false
	for _, r := range releases {
		if r.Barcode == "074646161425" {
			hasExpectedBarcode = true
			break
		}
	}
	if !hasExpectedBarcode {
		t.Error("barcode 074646161425 (2024 Digital edition) not found in releases")
	}

	defaultCount := 0
	for _, r := range releases {
		if r.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("expected exactly 1 IsDefault release, got %d", defaultCount)
	}
}

// TestMBZAdapter_Integration_BarcodeToRelease verifies that barcode 074646161425
// resolves to a Hi Infidelity release. Multiple pressings share this barcode;
// the test asserts the barcode is echoed back and the title matches rather than
// pinning a specific MBID (which varies by MBZ search ranking).
func TestMBZAdapter_Integration_BarcodeToRelease(t *testing.T) {
	a := newIntegrationAdapter()
	ctx, cancel := integrationCtx(t)
	defer cancel()

	rel, err := a.GetReleaseByBarcode(ctx, "074646161425")
	if err != nil {
		t.Fatalf("GetReleaseByBarcode: %v", err)
	}
	if rel == nil {
		t.Fatal("GetReleaseByBarcode returned nil for barcode 074646161425")
	}

	t.Logf("release: %s %q country=%s date=%s barcode=%s", rel.MBID, rel.Title, rel.Country, rel.Date, rel.Barcode)

	if rel.MBID == "" {
		t.Error("MBID is empty")
	}
	if !containsFold(rel.Title, "hi infidelity") {
		t.Errorf("Title = %q, expected to contain 'Hi Infidelity'", rel.Title)
	}
}

// containsFold is a case-insensitive substring check used in integration tests.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TestMBZ_FetchGroupContent verifies that tracks for a release-group can be
// retrieved with positive runtime durations. The release-group MBID is obtained
// dynamically from FetchEntryContent so this test doesn't hardcode a brittle ID.
func TestMBZ_FetchGroupContent(t *testing.T) {
	a := newIntegrationAdapter()

	// Step 1 — get any release-group MBID from The Beatles discography.
	ctx1, cancel1 := integrationCtx(t)
	defer cancel1()

	groups, _, _, err := a.FetchEntryContent(ctx1, domain.ContentTypeMusic, beatlesMBID, 1, 5)
	if err != nil {
		t.Fatalf("FetchEntryContent (setup): %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("FetchEntryContent returned no groups; cannot run FetchGroupContent")
	}

	// Step 2 — fetch tracks for the first release-group.
	ctx2, cancel2 := integrationCtx(t)
	defer cancel2()

	rgMBID := groups[0].ExternalID
	items, total, err := a.FetchGroupContent(ctx2, domain.ContentTypeMusic, rgMBID, 1, 50)
	if err != nil {
		t.Fatalf("FetchGroupContent(%q): %v", rgMBID, err)
	}
	if total == 0 {
		t.Errorf("total = 0 for release-group %q; expected tracks", rgMBID)
	}
	if len(items) == 0 {
		t.Fatalf("no items returned for release-group %q", rgMBID)
	}

	// At least one track must have a title and a positive runtime.
	hasRuntime := false
	for _, item := range items {
		if item.ExternalID == "" {
			t.Errorf("item.ExternalID is empty for track %q", item.Title)
		}
		if item.Title == "" {
			t.Errorf("item.Title is empty for ExternalID %q", item.ExternalID)
		}
		if item.RuntimeSecs > 0 {
			hasRuntime = true
		}
	}
	if !hasRuntime {
		t.Error("no track has RuntimeSecs > 0; MBZ recording length data must be present")
	}
}
