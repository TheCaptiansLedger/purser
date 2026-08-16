package apiconnect_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"purser/gen/go/purser/database/v1/databasev1connect"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	databasev1 "purser/gen/go/purser/database/v1"

	apiconnect "purser/internal/api/connect"
)

type fakeDatabaseService struct {
	info    ports.DatabaseInfo
	infoErr error

	backupWrites []byte
	backupErr    error

	restoreRead []byte
	restoreErr  error
}

func (f *fakeDatabaseService) Info(context.Context) (ports.DatabaseInfo, error) {
	if f.infoErr != nil {
		return ports.DatabaseInfo{}, f.infoErr
	}
	return f.info, nil
}

// Backup writes f.backupWrites in small pieces, the same way a real
// DatabaseAdmin adapter writes one JSONL line at a time — bufio.Writer
// only accumulates writes smaller than its buffer, so a single giant
// Write call would bypass chunking entirely and not exercise it.
func (f *fakeDatabaseService) Backup(_ context.Context, w io.Writer) error {
	if f.backupErr != nil {
		return f.backupErr
	}
	const pieceSize = 4096
	for i := 0; i < len(f.backupWrites); i += pieceSize {
		end := min(i+pieceSize, len(f.backupWrites))
		if _, err := w.Write(f.backupWrites[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDatabaseService) Restore(_ context.Context, r io.Reader) error {
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

// newDatabaseTestServer mounts h behind a real HTTP server — Backup's
// *connect.ServerStream and Restore's *connect.ClientStream have no
// exported constructor, so streaming handlers can only be exercised
// through a real client/server round-trip. See
// internal/api/connect/job_test.go's newWatchTestServer for the same
// convention.
func newDatabaseTestServer(t *testing.T, h *apiconnect.DatabaseHandler) databasev1connect.DatabaseServiceClient {
	t.Helper()
	path, handler := databasev1connect.NewDatabaseServiceHandler(h)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return databasev1connect.NewDatabaseServiceClient(server.Client(), server.URL)
}

func TestDatabaseHandler_GetDatabaseInfo(t *testing.T) {
	svc := &fakeDatabaseService{info: ports.DatabaseInfo{
		Driver:           "badger",
		Version:          "v4.9.4",
		StorageSizeBytes: 1024,
		CollectionCounts: map[string]int64{"person": 3},
	}}
	client := newDatabaseTestServer(t, apiconnect.NewDatabaseHandler(svc, nil))

	resp, err := client.GetDatabaseInfo(context.Background(), connect.NewRequest(&databasev1.GetDatabaseInfoRequest{}))
	if err != nil {
		t.Fatalf("GetDatabaseInfo returned error: %v", err)
	}
	if resp.Msg.GetDriver() != "badger" {
		t.Fatalf("GetDatabaseInfo returned Driver %q, want %q", resp.Msg.GetDriver(), "badger")
	}
	if resp.Msg.GetStorageSizeBytes() != 1024 {
		t.Fatalf("GetDatabaseInfo returned StorageSizeBytes %d, want 1024", resp.Msg.GetStorageSizeBytes())
	}
	if resp.Msg.GetCollectionCounts()["person"] != 3 {
		t.Fatalf("GetDatabaseInfo returned CollectionCounts[person] = %d, want 3", resp.Msg.GetCollectionCounts()["person"])
	}
}

func TestDatabaseHandler_GetDatabaseInfo_Error(t *testing.T) {
	svc := &fakeDatabaseService{infoErr: errors.New("boom")}
	client := newDatabaseTestServer(t, apiconnect.NewDatabaseHandler(svc, nil))

	_, err := client.GetDatabaseInfo(context.Background(), connect.NewRequest(&databasev1.GetDatabaseInfoRequest{}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("GetDatabaseInfo returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("GetDatabaseInfo returned code %v, want %v", connErr.Code(), connect.CodeInternal)
	}
}

func TestDatabaseHandler_Backup(t *testing.T) {
	want := bytes.Repeat([]byte("a"), 200*1024) // larger than one 64 KiB chunk
	svc := &fakeDatabaseService{backupWrites: want}
	client := newDatabaseTestServer(t, apiconnect.NewDatabaseHandler(svc, nil))

	stream, err := client.Backup(context.Background(), connect.NewRequest(&databasev1.BackupRequest{}))
	if err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}

	var got []byte
	chunks := 0
	for stream.Receive() {
		got = append(got, stream.Msg().GetData()...)
		chunks++
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream ended with error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("Backup streamed %d bytes, want %d matching bytes", len(got), len(want))
	}
	if chunks < 2 {
		t.Fatalf("Backup sent %d chunk(s), want more than 1 for a payload larger than one chunk", chunks)
	}
}

func TestDatabaseHandler_Backup_Error(t *testing.T) {
	svc := &fakeDatabaseService{backupErr: errors.New("boom")}
	client := newDatabaseTestServer(t, apiconnect.NewDatabaseHandler(svc, nil))

	stream, err := client.Backup(context.Background(), connect.NewRequest(&databasev1.BackupRequest{}))
	if err != nil {
		t.Fatalf("Backup returned error: %v", err)
	}
	if stream.Receive() {
		t.Fatal("stream.Receive() returned true, want the stream to end immediately with an error")
	}
	var connErr *connect.Error
	if !errors.As(stream.Err(), &connErr) {
		t.Fatalf("stream ended with %v, want a *connect.Error", stream.Err())
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("stream ended with code %v, want %v", connErr.Code(), connect.CodeInternal)
	}
}

func TestDatabaseHandler_Restore(t *testing.T) {
	svc := &fakeDatabaseService{}
	client := newDatabaseTestServer(t, apiconnect.NewDatabaseHandler(svc, nil))

	stream := client.Restore(context.Background())
	want := []byte(`{"purser_backup_version":1}` + "\n" + `{"collection":"widget","id":"w1","data":{"id":"w1"}}` + "\n")
	// Send in two pieces to exercise multi-chunk staging, not just a
	// single Send.
	if err := stream.Send(&databasev1.RestoreRequest{Data: want[:10]}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if err := stream.Send(&databasev1.RestoreRequest{Data: want[10:]}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	resp, err := stream.CloseAndReceive()
	if err != nil {
		t.Fatalf("CloseAndReceive returned error: %v", err)
	}
	if !resp.Msg.GetRestartRequired() {
		t.Fatalf("Restore response RestartRequired = false, want true")
	}
	if !bytes.Equal(svc.restoreRead, want) {
		t.Fatalf("service received %q, want %q", svc.restoreRead, want)
	}
}

func TestDatabaseHandler_Restore_Error(t *testing.T) {
	svc := &fakeDatabaseService{restoreErr: errors.New("invalid backup stream")}
	client := newDatabaseTestServer(t, apiconnect.NewDatabaseHandler(svc, nil))

	stream := client.Restore(context.Background())
	if err := stream.Send(&databasev1.RestoreRequest{Data: []byte("garbage")}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	_, err := stream.CloseAndReceive()
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("CloseAndReceive returned %v, want a *connect.Error", err)
	}
	if connErr.Code() != connect.CodeInternal {
		t.Fatalf("CloseAndReceive returned code %v, want %v", connErr.Code(), connect.CodeInternal)
	}
}
