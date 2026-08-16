package service_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeDatabaseAdmin struct {
	info    ports.DatabaseInfo
	infoErr error

	backupErr    error
	backupWrites []byte

	restoreErr  error
	restoreRead []byte
}

func (f *fakeDatabaseAdmin) Info(context.Context) (ports.DatabaseInfo, error) {
	if f.infoErr != nil {
		return ports.DatabaseInfo{}, f.infoErr
	}
	return f.info, nil
}

func (f *fakeDatabaseAdmin) Backup(_ context.Context, w io.Writer) error {
	if f.backupErr != nil {
		return f.backupErr
	}
	_, err := w.Write(f.backupWrites)
	return err
}

func (f *fakeDatabaseAdmin) Restore(_ context.Context, r io.Reader) error {
	if f.restoreErr != nil {
		return f.restoreErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.restoreRead = b
	return nil
}

func TestDatabaseService_Info(t *testing.T) {
	admin := &fakeDatabaseAdmin{info: ports.DatabaseInfo{Driver: "badger", Version: "v4.9.4"}}
	svc := service.NewDatabaseService(admin, nil)

	got, err := svc.Info(context.Background())
	if err != nil {
		t.Fatalf("Info returned error: %v", err)
	}
	if got.Driver != "badger" {
		t.Fatalf("Info returned Driver %q, want %q", got.Driver, "badger")
	}
}

func TestDatabaseService_Info_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	admin := &fakeDatabaseAdmin{infoErr: wantErr}
	svc := service.NewDatabaseService(admin, nil)

	if _, err := svc.Info(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Info returned error %v, want %v", err, wantErr)
	}
}

func TestDatabaseService_Backup(t *testing.T) {
	admin := &fakeDatabaseAdmin{backupWrites: []byte("backup-bytes")}
	svc := service.NewDatabaseService(admin, nil)

	var buf bytes.Buffer
	if err := svc.Backup(context.Background(), &buf); err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}
	if buf.String() != "backup-bytes" {
		t.Fatalf("Backup wrote %q, want %q", buf.String(), "backup-bytes")
	}
}

func TestDatabaseService_Restore_FiresOnRestoreOnSuccess(t *testing.T) {
	admin := &fakeDatabaseAdmin{}
	fired := false
	svc := service.NewDatabaseService(admin, func() { fired = true })

	if err := svc.Restore(context.Background(), bytes.NewReader([]byte("artifact"))); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if !fired {
		t.Fatalf("Restore did not call onRestore after a successful restore")
	}
	if string(admin.restoreRead) != "artifact" {
		t.Fatalf("Restore's port received %q, want %q", admin.restoreRead, "artifact")
	}
}

func TestDatabaseService_Restore_DoesNotFireOnRestoreOnFailure(t *testing.T) {
	admin := &fakeDatabaseAdmin{restoreErr: errors.New("invalid backup stream")}
	fired := false
	svc := service.NewDatabaseService(admin, func() { fired = true })

	if err := svc.Restore(context.Background(), bytes.NewReader(nil)); err == nil {
		t.Fatalf("Restore returned a nil error, want the port's error")
	}
	if fired {
		t.Fatalf("Restore called onRestore despite the port returning an error")
	}
}

func TestDatabaseService_Restore_NilOnRestoreIsSafe(t *testing.T) {
	admin := &fakeDatabaseAdmin{}
	svc := service.NewDatabaseService(admin, nil)

	if err := svc.Restore(context.Background(), bytes.NewReader(nil)); err != nil {
		t.Fatalf("Restore with a nil onRestore returned error: %v", err)
	}
}
