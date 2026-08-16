// Package database holds the backup/restore artifact format shared by
// every datastore.Datastore backend's ports.DatabaseAdmin adapter — see
// docs/technical/database-backup-restore.md. It is deliberately backend
// agnostic: internal/adapters/database/badger and .../sql both import it
// rather than each encoding/decoding the JSONL format themselves, the
// same adapter-to-adapter reuse internal/adapters/store.Repository[T]
// already established for entity translators (docs/adr/0012-datastore-persistence.md).
package database

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"purser/internal/adapters/datastore"
)

// FormatVersion is the current backup artifact version, stamped as the
// first line of every backup stream.
const FormatVersion = 1

// ReplayBatchSize bounds how many documents Replay hands to CreateBatch
// at a time, per docs/adr/0016-bulk-operations.md and the design doc's
// §3 ("bounded-size chunks (e.g. 500 documents)").
const ReplayBatchSize = 500

// maxLineBytes bounds how large a single JSONL line (one document, or the
// header) Validate/Replay will accept — bufio.Scanner's own default
// (64 KiB) is too small for a document carrying a sizeable payload
// (e.g. an Image's encoded bytes).
const maxLineBytes = 16 << 20

// headerLine is the JSON shape of a backup artifact's first line.
type headerLine struct {
	Version int `json:"purser_backup_version"`
}

// docLine is the JSON shape of every subsequent line — the wire form of a
// datastore.Document. See docs/technical/database-backup-restore.md §2.1.
type docLine struct {
	Collection string            `json:"collection"`
	ID         string            `json:"id"`
	Data       json.RawMessage   `json:"data"`
	Index      map[string]string `json:"index,omitempty"`
}

// WriteHeader writes the version-header line every backup artifact starts
// with.
func WriteHeader(w io.Writer) error {
	return writeLine(w, headerLine{Version: FormatVersion})
}

// WriteDocument writes one datastore.Document as a single JSONL line.
func WriteDocument(w io.Writer, doc datastore.Document) error {
	return writeLine(w, docLine{Collection: doc.Collection, ID: doc.ID, Data: json.RawMessage(doc.Data), Index: doc.Index})
}

func writeLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("database: encode line: %w", err)
	}
	b = append(b, '\n')
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("database: write line: %w", err)
	}
	return nil
}

func newScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	return sc
}

// readHeader reads and validates the stream's first line as a version
// header, rejecting anything that isn't exactly FormatVersion — a
// mismatch means either a much older or much newer Purser wrote the
// stream, per the design doc's §2.1.
func readHeader(sc *bufio.Scanner) error {
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return fmt.Errorf("database: reading header: %w", err)
		}
		return fmt.Errorf("database: empty backup stream, expected a version header")
	}
	var h headerLine
	if err := json.Unmarshal(sc.Bytes(), &h); err != nil {
		return fmt.Errorf("database: decoding header line: %w", err)
	}
	if h.Version != FormatVersion {
		return fmt.Errorf("database: backup format version %d, this build only supports %d", h.Version, FormatVersion)
	}
	return nil
}

// Validate performs a full pass over r, confirming the version header and
// every subsequent line decodes as a valid document — without touching
// any datastore. Returns a line-numbered error describing the first
// problem found.
func Validate(r io.Reader) error {
	sc := newScanner(r)
	if err := readHeader(sc); err != nil {
		return err
	}
	line := 1
	for sc.Scan() {
		line++
		var d docLine
		if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
			return fmt.Errorf("database: line %d: invalid document: %w", line, err)
		}
		if d.Collection == "" || d.ID == "" {
			return fmt.Errorf("database: line %d: document missing collection or id", line)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("database: validating backup stream: %w", err)
	}
	return nil
}

// Replay decodes every document line in r and hands bounded
// ReplayBatchSize chunks to ds.CreateBatch, per the design doc's §3 — the
// only write-path code restore needs, reusing CreateBatch's existing
// per-chunk atomicity. r must start at its version header; Replay
// consumes and validates it like Validate does.
func Replay(ctx context.Context, r io.Reader, ds datastore.Datastore) error {
	sc := newScanner(r)
	if err := readHeader(sc); err != nil {
		return err
	}

	batch := make([]datastore.Document, 0, ReplayBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := ds.CreateBatch(ctx, batch); err != nil {
			return fmt.Errorf("database: replaying batch: %w", err)
		}
		batch = batch[:0]
		return nil
	}

	line := 1
	for sc.Scan() {
		line++
		var d docLine
		if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
			return fmt.Errorf("database: line %d: invalid document: %w", line, err)
		}
		batch = append(batch, datastore.Document{Collection: d.Collection, ID: d.ID, Data: []byte(d.Data), Index: d.Index})
		if len(batch) == ReplayBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("database: replaying backup stream: %w", err)
	}
	return flush()
}

// ApplyRestore is the shared Restore implementation both backend adapters
// call: if r is also an io.Seeker (the Connect handler always hands
// adapters an *os.File it has already staged the upload into — see
// internal/api/connect/database.go), ApplyRestore validates the entire
// stream first and rewinds before calling clear (the backend's own
// destructive wipe) and Replay — never touching the datastore on a
// stream that turns out truncated or malformed. If r isn't seekable it's
// replayed directly with no validation pass; every real caller in this
// codebase hands ApplyRestore a seekable *os.File, per
// docs/technical/database-backup-restore.md's staged-apply design.
func ApplyRestore(ctx context.Context, r io.Reader, ds datastore.Datastore, clear func(ctx context.Context) error) error {
	if seeker, ok := r.(io.Seeker); ok {
		if err := Validate(r); err != nil {
			return fmt.Errorf("database: restore rejected: %w", err)
		}
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("database: rewinding validated backup stream: %w", err)
		}
	}
	if err := clear(ctx); err != nil {
		return fmt.Errorf("database: clearing datastore before restore: %w", err)
	}
	return Replay(ctx, r, ds)
}
