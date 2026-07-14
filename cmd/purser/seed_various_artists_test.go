package main

import (
	"context"
	"errors"
	"fmt"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"testing"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
)

// fakeLibraryEntryClient is a minimal in-memory
// domainv1connect.LibraryEntryServiceClient, exercising only the RPCs
// runSeedVariousArtists actually calls.
type fakeLibraryEntryClient struct {
	domainv1connect.LibraryEntryServiceClient
	entries map[string]*v1.LibraryEntry
	nextID  int
}

func newFakeLibraryEntryClient() *fakeLibraryEntryClient {
	return &fakeLibraryEntryClient{entries: make(map[string]*v1.LibraryEntry)}
}

func (f *fakeLibraryEntryClient) CreateLibraryEntry(_ context.Context, req *connect.Request[v1.CreateLibraryEntryRequest]) (*connect.Response[v1.CreateLibraryEntryResponse], error) {
	f.nextID++
	id := fmt.Sprintf("le-%d", f.nextID)
	in := req.Msg.GetLibraryEntry()
	e := &v1.LibraryEntry{
		Id:          id,
		ContentType: in.GetContentType(),
		Kind:        in.GetKind(),
		Name:        in.GetName(),
		MonitorMode: in.GetMonitorMode(),
	}
	f.entries[id] = e
	return connect.NewResponse(&v1.CreateLibraryEntryResponse{LibraryEntry: e}), nil
}

func (f *fakeLibraryEntryClient) ListLibraryEntries(_ context.Context, req *connect.Request[v1.ListLibraryEntriesRequest]) (*connect.Response[v1.ListLibraryEntriesResponse], error) {
	var out []*v1.LibraryEntry
	for _, e := range f.entries {
		if kind := req.Msg.GetKind(); kind != "" && e.GetKind() != kind {
			continue
		}
		out = append(out, e)
	}
	return connect.NewResponse(&v1.ListLibraryEntriesResponse{LibraryEntries: out}), nil
}

// fakeExternalIDClient is a minimal in-memory
// domainv1connect.ExternalIDServiceClient, exercising only the RPCs
// runSeedVariousArtists actually calls.
type fakeExternalIDClient struct {
	domainv1connect.ExternalIDServiceClient
	byKey map[string]*v1.ExternalID
}

func newFakeExternalIDClient() *fakeExternalIDClient {
	return &fakeExternalIDClient{byKey: make(map[string]*v1.ExternalID)}
}

func externalIDKey(entityType v1.EntityType, entityID, source string) string {
	return fmt.Sprintf("%v|%s|%s", entityType, entityID, source)
}

func (f *fakeExternalIDClient) GetExternalID(_ context.Context, req *connect.Request[v1.GetExternalIDRequest]) (*connect.Response[v1.GetExternalIDResponse], error) {
	e, ok := f.byKey[externalIDKey(req.Msg.GetEntityType(), req.Msg.GetEntityId(), req.Msg.GetSource())]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("external id not found"))
	}
	return connect.NewResponse(&v1.GetExternalIDResponse{ExternalId: e}), nil
}

func (f *fakeExternalIDClient) CreateExternalID(_ context.Context, req *connect.Request[v1.CreateExternalIDRequest]) (*connect.Response[v1.CreateExternalIDResponse], error) {
	e := req.Msg.GetExternalId()
	key := externalIDKey(e.GetEntityType(), e.GetEntityId(), e.GetSource())
	if _, exists := f.byKey[key]; exists {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("external id already exists"))
	}
	f.byKey[key] = e
	return connect.NewResponse(&v1.CreateExternalIDResponse{ExternalId: e}), nil
}

func TestRunSeedVariousArtists_CreatesWhenMissing(t *testing.T) {
	entryClient := newFakeLibraryEntryClient()
	idClient := newFakeExternalIDClient()

	if err := runSeedVariousArtists(context.Background(), entryClient, idClient); err != nil {
		t.Fatalf("runSeedVariousArtists returned error: %v", err)
	}

	if len(entryClient.entries) != 1 {
		t.Fatalf("got %d library entries, want 1", len(entryClient.entries))
	}
	var entry *v1.LibraryEntry
	for _, e := range entryClient.entries {
		entry = e
	}
	if entry.GetName() != variousArtistsName || entry.GetKind() != "artist" || entry.GetContentType() != "music" {
		t.Fatalf("created entry %+v, want Name=%q Kind=artist ContentType=music", entry, variousArtistsName)
	}
	if entry.GetMonitorMode() != v1.MonitorMode_MONITOR_MODE_NONE {
		t.Fatalf("created entry MonitorMode = %v, want MONITOR_MODE_NONE", entry.GetMonitorMode())
	}

	if len(idClient.byKey) != 1 {
		t.Fatalf("got %d external ids, want 1", len(idClient.byKey))
	}
	for _, id := range idClient.byKey {
		if id.GetEntityId() != entry.GetId() || id.GetSource() != "mbz" || id.GetValue() != variousArtistsMBID {
			t.Fatalf("created external id %+v, want EntityId=%q Source=mbz Value=%q", id, entry.GetId(), variousArtistsMBID)
		}
	}
}

func TestRunSeedVariousArtists_SecondRunIsANoOp(t *testing.T) {
	entryClient := newFakeLibraryEntryClient()
	idClient := newFakeExternalIDClient()

	if err := runSeedVariousArtists(context.Background(), entryClient, idClient); err != nil {
		t.Fatalf("first run returned error: %v", err)
	}
	if err := runSeedVariousArtists(context.Background(), entryClient, idClient); err != nil {
		t.Fatalf("second run returned error: %v", err)
	}

	if len(entryClient.entries) != 1 {
		t.Fatalf("after two runs got %d library entries, want exactly 1", len(entryClient.entries))
	}
	if len(idClient.byKey) != 1 {
		t.Fatalf("after two runs got %d external ids, want exactly 1", len(idClient.byKey))
	}
}

func TestRunSeedVariousArtists_ReusesExistingEntryByName(t *testing.T) {
	entryClient := newFakeLibraryEntryClient()
	entryClient.entries["existing"] = &v1.LibraryEntry{
		Id:          "existing",
		ContentType: "music",
		Kind:        "artist",
		Name:        variousArtistsName,
		MonitorMode: v1.MonitorMode_MONITOR_MODE_NONE,
	}
	idClient := newFakeExternalIDClient()

	if err := runSeedVariousArtists(context.Background(), entryClient, idClient); err != nil {
		t.Fatalf("runSeedVariousArtists returned error: %v", err)
	}

	if len(entryClient.entries) != 1 {
		t.Fatalf("got %d library entries, want the pre-existing 1 reused, not a duplicate", len(entryClient.entries))
	}
	id, ok := idClient.byKey[externalIDKey(v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY, "existing", "mbz")]
	if !ok {
		t.Fatalf("expected an mbz external id attached to the pre-existing entry")
	}
	if id.GetValue() != variousArtistsMBID {
		t.Fatalf("external id Value = %q, want %q", id.GetValue(), variousArtistsMBID)
	}
}

func TestRunSeedVariousArtists_ConflictingExternalIDIsAnError(t *testing.T) {
	entryClient := newFakeLibraryEntryClient()
	entryClient.entries["existing"] = &v1.LibraryEntry{
		Id: "existing", ContentType: "music", Kind: "artist", Name: variousArtistsName,
		MonitorMode: v1.MonitorMode_MONITOR_MODE_NONE,
	}
	idClient := newFakeExternalIDClient()
	idClient.byKey[externalIDKey(v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY, "existing", "mbz")] = &v1.ExternalID{
		EntityType: v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY, EntityId: "existing", Source: "mbz", Value: "some-other-mbid",
	}

	err := runSeedVariousArtists(context.Background(), entryClient, idClient)
	if err == nil {
		t.Fatal("runSeedVariousArtists with a conflicting pre-existing external id returned nil, want an error")
	}
}
