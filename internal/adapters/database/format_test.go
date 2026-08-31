package database_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"purser/internal/adapters/database"
	"purser/internal/adapters/datastore"
	"testing"

	dsbadger "purser/internal/adapters/datastore/badger"
)

// newTestDatastore returns a real, empty datastore.Datastore backed by a
// temp-dir BadgerDB instance — the same "real backend, no mocking"
// convention every other adapter test in this codebase follows (see
// internal/adapters/database/badger/admin_test.go).
func newTestDatastore(t *testing.T) datastore.Datastore {
	t.Helper()

	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("dsbadger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})

	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badgerstore.New returned error: %v", err)
	}
	return ds
}

func TestWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if got, want := buf.String(), "{\"purser_backup_version\":1}\n"; got != want {
		t.Fatalf("WriteHeader wrote %q, want %q", got, want)
	}
}

func TestWriteHeader_PropagatesWriteError(t *testing.T) {
	if err := database.WriteHeader(failingWriter{}); err == nil {
		t.Fatalf("WriteHeader with a failing writer returned a nil error")
	}
}

func TestWriteDocument(t *testing.T) {
	var buf bytes.Buffer
	doc := datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1"}`), Index: map[string]string{"owner": "alice"}}
	if err := database.WriteDocument(&buf, doc); err != nil {
		t.Fatalf("WriteDocument returned error: %v", err)
	}
	if got, want := buf.String(), "{\"collection\":\"widget\",\"id\":\"w1\",\"data\":{\"id\":\"w1\"},\"index\":{\"owner\":\"alice\"}}\n"; got != want {
		t.Fatalf("WriteDocument wrote %q, want %q", got, want)
	}
}

func TestWriteDocument_PropagatesWriteError(t *testing.T) {
	doc := datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{}`)}
	if err := database.WriteDocument(failingWriter{}, doc); err == nil {
		t.Fatalf("WriteDocument with a failing writer returned a nil error")
	}
}

func TestValidate_AcceptsAWellFormedStream(t *testing.T) {
	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if err := database.WriteDocument(&buf, datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{}`)}); err != nil {
		t.Fatalf("WriteDocument returned error: %v", err)
	}
	if err := database.Validate(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidate_RejectsAnEmptyStream(t *testing.T) {
	if err := database.Validate(bytes.NewReader(nil)); err == nil {
		t.Fatalf("Validate of an empty stream returned a nil error")
	}
}

func TestValidate_RejectsAReadError(t *testing.T) {
	if err := database.Validate(erroringReader{}); err == nil {
		t.Fatalf("Validate of a reader that errors returned a nil error")
	}
}

func TestValidate_RejectsAMalformedHeader(t *testing.T) {
	if err := database.Validate(bytes.NewReader([]byte("not json\n"))); err == nil {
		t.Fatalf("Validate with a malformed header line returned a nil error")
	}
}

func TestValidate_RejectsAWrongVersionHeader(t *testing.T) {
	if err := database.Validate(bytes.NewReader([]byte(`{"purser_backup_version":999}` + "\n"))); err == nil {
		t.Fatalf("Validate with a wrong version header returned a nil error")
	}
}

func TestValidate_RejectsAMalformedDocumentLine(t *testing.T) {
	stream := []byte(`{"purser_backup_version":1}` + "\nnot json\n")
	if err := database.Validate(bytes.NewReader(stream)); err == nil {
		t.Fatalf("Validate with a malformed document line returned a nil error")
	}
}

func TestValidate_RejectsADocumentMissingCollectionOrID(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"missing collection", `{"id":"w1","data":{}}`},
		{"missing id", `{"collection":"widget","data":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := []byte(`{"purser_backup_version":1}` + "\n" + tt.line + "\n")
			if err := database.Validate(bytes.NewReader(stream)); err == nil {
				t.Fatalf("Validate with a document %s returned a nil error", tt.name)
			}
		})
	}
}

func TestReplay_WritesEveryDocumentToTheDatastore(t *testing.T) {
	ds := newTestDatastore(t)

	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	docs := []datastore.Document{
		{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1"}`)},
		{Collection: "widget", ID: "w2", Data: []byte(`{"id":"w2"}`), Index: map[string]string{"owner": "alice"}},
	}
	for _, d := range docs {
		if err := database.WriteDocument(&buf, d); err != nil {
			t.Fatalf("WriteDocument returned error: %v", err)
		}
	}

	if err := database.Replay(context.Background(), bytes.NewReader(buf.Bytes()), ds); err != nil {
		t.Fatalf("Replay returned error: %v", err)
	}

	got, err := ds.Get(context.Background(), "widget", "w2")
	if err != nil {
		t.Fatalf("Get after Replay returned error: %v", err)
	}
	if got.Index["owner"] != "alice" {
		t.Fatalf("Get after Replay returned Index %v, want owner=alice", got.Index)
	}
}

func TestReplay_RejectsAWrongVersionHeader(t *testing.T) {
	ds := newTestDatastore(t)
	stream := []byte(`{"purser_backup_version":999}` + "\n")
	if err := database.Replay(context.Background(), bytes.NewReader(stream), ds); err == nil {
		t.Fatalf("Replay with a wrong version header returned a nil error")
	}
}

func TestReplay_RejectsAMalformedDocumentLine(t *testing.T) {
	ds := newTestDatastore(t)
	stream := []byte(`{"purser_backup_version":1}` + "\nnot json\n")
	if err := database.Replay(context.Background(), bytes.NewReader(stream), ds); err == nil {
		t.Fatalf("Replay with a malformed document line returned a nil error")
	}
}

func TestReplay_RejectsAReadError(t *testing.T) {
	ds := newTestDatastore(t)
	if err := database.Replay(context.Background(), erroringReader{}, ds); err == nil {
		t.Fatalf("Replay of a reader that errors returned a nil error")
	}
}

func TestReplay_PropagatesADatastoreError(t *testing.T) {
	ds := newTestDatastore(t)

	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	// Two documents that collide on Collection+ID make the underlying
	// CreateBatch conflict, which Replay must surface rather than swallow.
	doc := datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1"}`)}
	if err := database.WriteDocument(&buf, doc); err != nil {
		t.Fatalf("WriteDocument returned error: %v", err)
	}
	if err := database.WriteDocument(&buf, doc); err != nil {
		t.Fatalf("WriteDocument returned error: %v", err)
	}

	if err := database.Replay(context.Background(), bytes.NewReader(buf.Bytes()), ds); err == nil {
		t.Fatalf("Replay of a batch with a duplicate ID returned a nil error")
	}
}

func TestApplyRestore_ValidatesRewindsAndReplaysASeekableStream(t *testing.T) {
	ds := newTestDatastore(t)
	if err := ds.Create(context.Background(), datastore.Document{Collection: "widget", ID: "stale", Data: []byte(`{}`)}); err != nil {
		t.Fatalf("seeding stale document: Create returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if err := database.WriteDocument(&buf, datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1"}`)}); err != nil {
		t.Fatalf("WriteDocument returned error: %v", err)
	}

	cleared := false
	clear := func(ctx context.Context) error {
		cleared = true
		return ds.Delete(ctx, "widget", "stale")
	}

	if err := database.ApplyRestore(context.Background(), bytes.NewReader(buf.Bytes()), ds, clear); err != nil {
		t.Fatalf("ApplyRestore returned error: %v", err)
	}
	if !cleared {
		t.Fatalf("ApplyRestore did not call clear for a seekable, valid stream")
	}

	if _, err := ds.Get(context.Background(), "widget", "stale"); err == nil {
		t.Fatalf("stale document survived ApplyRestore")
	}
	if _, err := ds.Get(context.Background(), "widget", "w1"); err != nil {
		t.Fatalf("Get w1 after ApplyRestore returned error: %v", err)
	}
}

func TestApplyRestore_RejectsAnInvalidSeekableStreamWithoutClearing(t *testing.T) {
	ds := newTestDatastore(t)
	stream := []byte(`{"purser_backup_version":999}` + "\n")

	clear := func(context.Context) error {
		t.Fatalf("ApplyRestore called clear for an invalid stream")
		return nil
	}

	if err := database.ApplyRestore(context.Background(), bytes.NewReader(stream), ds, clear); err == nil {
		t.Fatalf("ApplyRestore with a wrong version header returned a nil error")
	}
}

func TestApplyRestore_PropagatesAClearError(t *testing.T) {
	ds := newTestDatastore(t)

	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}

	wantErr := errors.New("clear boom")
	clear := func(context.Context) error { return wantErr }

	err := database.ApplyRestore(context.Background(), bytes.NewReader(buf.Bytes()), ds, clear)
	if err == nil {
		t.Fatalf("ApplyRestore with a failing clear returned a nil error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("ApplyRestore error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestApplyRestore_ReplaysANonSeekableStreamWithoutValidating(t *testing.T) {
	ds := newTestDatastore(t)

	var buf bytes.Buffer
	if err := database.WriteHeader(&buf); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if err := database.WriteDocument(&buf, datastore.Document{Collection: "widget", ID: "w1", Data: []byte(`{"id":"w1"}`)}); err != nil {
		t.Fatalf("WriteDocument returned error: %v", err)
	}

	cleared := false
	clear := func(context.Context) error {
		cleared = true
		return nil
	}

	// io.NopCloser strips the io.Seeker interface a bytes.Reader would
	// otherwise expose, exercising ApplyRestore's non-seekable branch —
	// no Validate pass, straight to clear+Replay.
	nonSeekable := io.NopCloser(bytes.NewReader(buf.Bytes()))
	if err := database.ApplyRestore(context.Background(), nonSeekable, ds, clear); err != nil {
		t.Fatalf("ApplyRestore returned error: %v", err)
	}
	if !cleared {
		t.Fatalf("ApplyRestore did not call clear for a non-seekable stream")
	}
	if _, err := ds.Get(context.Background(), "widget", "w1"); err != nil {
		t.Fatalf("Get w1 after ApplyRestore returned error: %v", err)
	}
}

// failingWriter always returns an error from Write, exercising the
// write-error propagation path in WriteHeader/WriteDocument.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

// erroringReader always returns a non-EOF error from Read, exercising the
// bufio.Scanner error path (as opposed to a clean EOF on an empty stream).
type erroringReader struct{}

func (erroringReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }
