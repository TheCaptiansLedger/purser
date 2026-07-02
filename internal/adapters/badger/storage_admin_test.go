package badger_test

import (
	"bytes"
	"context"
	"purser/internal/adapters/badger"
	"purser/internal/config"
	"purser/internal/domain"
	"strings"
	"testing"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
)

// openWithDir opens a BadgerDB in dir and returns the DB. The caller is
// responsible for closing it.
func openWithDir(t *testing.T, dir string) *badgerdb.DB {
	t.Helper()
	opts := badgerdb.DefaultOptions(dir).WithLogger(nil)
	db, err := badgerdb.Open(opts)
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	return db
}

func TestBadger_StorageAdmin_DriverName(t *testing.T) {
	db := setupTestDB(t)
	admin := badger.NewStorageAdmin(db, t.TempDir())
	if admin.DriverName() != "badger" {
		t.Errorf("DriverName = %q, want badger", admin.DriverName())
	}
}

func TestBadger_StorageAdmin_Stats_Empty(t *testing.T) {
	dir := t.TempDir()
	db := openWithDir(t, dir)
	t.Cleanup(func() { db.Close() })

	admin := badger.NewStorageAdmin(db, dir)
	stats, err := admin.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Driver != "badger" {
		t.Errorf("Driver = %q, want badger", stats.Driver)
	}
	if len(stats.Collections) != 7 {
		t.Errorf("Collections count = %d, want 7", len(stats.Collections))
	}
	for _, c := range stats.Collections {
		if c.Count != 0 {
			t.Errorf("collection %q count = %d, want 0 on empty db", c.Name, c.Count)
		}
	}
}

func TestBadger_StorageAdmin_Stats_WithData(t *testing.T) {
	dir := t.TempDir()
	db := openWithDir(t, dir)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	entryRepo := badger.NewLibraryEntryRepo(db)
	personRepo := badger.NewPersonRepo(db)
	tagRepo := badger.NewTagRepo(db)

	entryRepo.Save(ctx, &domain.LibraryEntry{ //nolint:errcheck
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Studio A", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	})
	entryRepo.Save(ctx, &domain.LibraryEntry{ //nolint:errcheck
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Studio B", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	})
	personRepo.Save(ctx, &domain.Person{Name: "Jane Doe", MonitorMode: domain.MonitorAll}) //nolint:errcheck
	tagRepo.Save(ctx, &domain.Tag{Key: "genre", Value: "Action", Scope: "adult"})          //nolint:errcheck

	admin := badger.NewStorageAdmin(db, dir)
	stats, err := admin.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	counts := make(map[string]int64)
	for _, c := range stats.Collections {
		counts[c.Name] = c.Count
	}

	if counts["library_entries"] != 2 {
		t.Errorf("library_entries count = %d, want 2", counts["library_entries"])
	}
	if counts["people"] != 1 {
		t.Errorf("people count = %d, want 1", counts["people"])
	}
	if counts["tags"] != 1 {
		t.Errorf("tags count = %d, want 1", counts["tags"])
	}
	if stats.SizeBytes == 0 {
		t.Error("SizeBytes should be non-zero for a DB with data on disk")
	}
	if stats.DriverVersion == "" {
		t.Error("DriverVersion should not be empty")
	}
	if stats.Extra == nil {
		t.Error("Extra should not be nil")
	}
	if _, ok := stats.Extra["lsm_size_bytes"]; !ok {
		t.Error("Extra should contain lsm_size_bytes")
	}
	if _, ok := stats.Extra["vlog_size_bytes"]; !ok {
		t.Error("Extra should contain vlog_size_bytes")
	}
}

func TestBadger_StorageAdmin_BackupRestore_RoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	srcDB, err := badger.Open(config.BadgerConfig{DataDir: srcDir})
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}

	ctx := context.Background()
	entryRepo := badger.NewLibraryEntryRepo(srcDB)

	entry := &domain.LibraryEntry{
		ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio,
		Name: "Restored Studio", MonitorMode: domain.MonitorAll, Status: domain.EntryStatusActive,
	}
	if err := entryRepo.Save(ctx, entry); err != nil {
		t.Fatalf("Save entry: %v", err)
	}

	srcAdmin := badger.NewStorageAdmin(srcDB, srcDir)

	var buf bytes.Buffer
	if err := srcAdmin.Backup(ctx, &buf); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("Backup produced empty output")
	}

	// Restore into a fresh database directory (must be a sibling of srcDir so
	// os.Rename works across the same parent — MkdirTemp handles this).
	dstDir := t.TempDir()
	dstDB := openWithDir(t, dstDir)

	shutdownCh := make(chan struct{}, 1)
	dstAdmin := badger.NewStorageAdmin(dstDB, dstDir)

	// Restore closes dstDB internally, so we must not call dstDB.Close() again.
	stats, err := dstAdmin.Restore(ctx, &buf, func() { shutdownCh <- struct{}{} })
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	select {
	case <-shutdownCh:
	case <-time.After(2 * time.Second):
		t.Error("shutdownFn was not called within 2 seconds after restore")
	}

	counts := make(map[string]int64)
	for _, c := range stats.Collections {
		counts[c.Name] = c.Count
	}
	if counts["library_entries"] != 1 {
		t.Errorf("restored library_entries count = %d, want 1", counts["library_entries"])
	}

	// Re-open the restored directory and verify the data is accessible.
	restoredDB := openWithDir(t, dstDir)
	t.Cleanup(func() { restoredDB.Close() })

	restoredRepo := badger.NewLibraryEntryRepo(restoredDB)
	got, err := restoredRepo.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get after restore: %v", err)
	}
	if got.Name != "Restored Studio" {
		t.Errorf("restored entry name = %q, want Restored Studio", got.Name)
	}
}

func TestBadger_StorageAdmin_Restore_RejectsSQLDump(t *testing.T) {
	dir := t.TempDir()
	db := openWithDir(t, dir)
	// Do not defer db.Close() — Restore may close it if detection happens after open.
	// In this case detection happens on the first 2 bytes, before any DB ops.
	t.Cleanup(func() { db.Close() }) //nolint:errcheck

	admin := badger.NewStorageAdmin(db, dir)
	sqlDump := strings.NewReader("-- Purser database dump\nBEGIN TRANSACTION;\nCOMMIT;")

	_, err := admin.Restore(context.Background(), sqlDump, func() {})
	if err == nil {
		t.Fatal("Restore should have returned an error for a SQL dump")
	}
	if !strings.Contains(err.Error(), "INVALID_FORMAT") {
		t.Errorf("error should mention INVALID_FORMAT, got: %v", err)
	}
}
