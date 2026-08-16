package apiconnect

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"purser/gen/go/purser/database/v1/databasev1connect"
	"purser/internal/ports"

	"connectrpc.com/connect"

	databasev1 "purser/gen/go/purser/database/v1"
)

// backupChunkSize bounds how many bytes Backup accumulates before sending
// one BackupChunk — independent of however many individual Write calls
// the DatabaseAdmin adapter makes while walking the backend.
const backupChunkSize = 64 * 1024

// databaseService is the narrow interface DatabaseHandler depends on —
// see personService for the DIP convention this follows.
type databaseService interface {
	Info(ctx context.Context) (ports.DatabaseInfo, error)
	Backup(ctx context.Context, w io.Writer) error
	Restore(ctx context.Context, r io.Reader) error
}

// DatabaseHandler implements databasev1connect.DatabaseServiceHandler — a
// thin request/response translator per docs/adr/0011-api-design.md, zero
// business logic. See docs/technical/database-backup-restore.md.
type DatabaseHandler struct {
	databasev1connect.UnimplementedDatabaseServiceHandler
	svc    databaseService
	logger *slog.Logger
}

// NewDatabaseHandler constructs a DatabaseHandler backed by svc.
func NewDatabaseHandler(svc databaseService, logger *slog.Logger) *DatabaseHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DatabaseHandler{svc: svc, logger: logger.With("component", "api.connect", "service", "DatabaseService")}
}

// GetDatabaseInfo implements databasev1connect.DatabaseServiceHandler.
func (h *DatabaseHandler) GetDatabaseInfo(ctx context.Context, _ *connect.Request[databasev1.GetDatabaseInfoRequest]) (*connect.Response[databasev1.GetDatabaseInfoResponse], error) {
	info, err := h.svc.Info(ctx)
	if err != nil {
		return nil, mapError(ctx, h.logger, err)
	}
	return connect.NewResponse(databaseInfoToProto(info)), nil
}

// backupStreamWriter adapts a *connect.ServerStream[BackupChunk] to
// io.Writer, so the service/port layer can write a backup artifact
// without knowing streaming RPCs exist.
type backupStreamWriter struct {
	stream *connect.ServerStream[databasev1.BackupChunk]
}

func (w *backupStreamWriter) Write(p []byte) (int, error) {
	// p is only valid until Write returns and bufio.Writer reuses its
	// buffer, so the chunk must be copied before Send, which may retain
	// the message past this call.
	data := make([]byte, len(p))
	copy(data, p)
	if err := w.stream.Send(&databasev1.BackupChunk{Data: data}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Backup implements databasev1connect.DatabaseServiceHandler. The
// backupChunkWriter is wrapped in a bufio.Writer so chunk size is
// independent of however many individual Write calls the DatabaseAdmin
// adapter underneath makes while walking the backend.
func (h *DatabaseHandler) Backup(ctx context.Context, _ *connect.Request[databasev1.BackupRequest], stream *connect.ServerStream[databasev1.BackupChunk]) error {
	bw := bufio.NewWriterSize(&backupStreamWriter{stream: stream}, backupChunkSize)
	if err := h.svc.Backup(ctx, bw); err != nil {
		return mapError(ctx, h.logger, err)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	h.logger.InfoContext(ctx, "database backup streamed to client")
	return nil
}

// Restore implements databasev1connect.DatabaseServiceHandler — this
// codebase's first client-streaming RPC. It stages the full upload to a
// temp file before calling into the service/port layer at all: the
// service and port only ever see a plain, already-materialized,
// seekable io.Reader (an *os.File), never a live wire stream — see
// docs/technical/database-backup-restore.md's staged-apply design. This
// is deliberately where that staging lives, not in internal/service:
// adapting an incoming proto stream into a single reader is protocol
// translation, the driving adapter's job, and keeps internal/service
// free of filesystem concerns.
func (h *DatabaseHandler) Restore(ctx context.Context, stream *connect.ClientStream[databasev1.RestoreRequest]) (*connect.Response[databasev1.RestoreResponse], error) {
	tmp, err := os.CreateTemp("", "purser-restore-*.jsonl")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	for stream.Receive() {
		if _, err := tmp.Write(stream.Msg().GetData()); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := h.svc.Restore(ctx, tmp); err != nil {
		return nil, mapError(ctx, h.logger, err)
	}

	h.logger.InfoContext(ctx, "database restore applied, process will exit for restart")
	return connect.NewResponse(&databasev1.RestoreResponse{RestartRequired: true}), nil
}
