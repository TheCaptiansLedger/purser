package ports

import (
	"context"
	"purser/internal/domain"
)

// ScanFilter narrows what the scanner emits.
// Empty ContentTypes means all registered content types.
// Non-empty EntryID restricts to roots for that library entry only.
type ScanFilter struct {
	ContentTypes []domain.ContentType
	EntryID      string
}

// FileScanner walks configured roots and emits candidate files.
// Skips files already present in MediaFileRepository at the same path and size.
// Skips extensions not in the media set for the file's content type.
type FileScanner interface {
	Scan(ctx context.Context, roots []string, f ScanFilter) (<-chan domain.ScannedFile, error)
}

// WatchOp describes what happened to a watched path.
type WatchOp int

// Watch operation types describing what happened to a watched path.
const (
	WatchCreated WatchOp = iota
	WatchModified
	WatchRemoved
)

// WatchEvent is emitted by FileWatcher when a file settles after creation or modification.
type WatchEvent struct {
	Path        string
	ContentType domain.ContentType
	Size        int64
	Op          WatchOp
}

// FileWatcher emits events when files appear, change, or disappear under watched roots.
// Implementations must debounce rapid write events so that a large file copy emits
// exactly one Created event after the write stabilises.
type FileWatcher interface {
	Watch(ctx context.Context, roots []string) (<-chan WatchEvent, error)
}

// FileFingerprinter computes fingerprints for a scanned file.
// Each adapter declares the content types it handles via ContentTypes().
// The scan service fans out to all registered fingerprinters, accumulating results.
// Returns nil, nil for unsupported content types.
type FileFingerprinter interface {
	ContentTypes() []domain.ContentType
	Fingerprint(ctx context.Context, f domain.ScannedFile) (*domain.Fingerprint, error)
}

// FileIdentifier matches a fingerprinted ScannedFile against library items.
// Each adapter declares the content types it handles via ContentTypes().
// The scan service fans out to all registered identifiers and merges candidates.
// Implementations must not mutate the Fingerprint on the ScannedFile.
type FileIdentifier interface {
	ContentTypes() []domain.ContentType
	Identify(ctx context.Context, f domain.ScannedFile) ([]domain.MatchCandidate, error)
}

// UnmatchedFilter narrows the unmatched file queue listing.
type UnmatchedFilter struct {
	ContentType domain.ContentType
	Status      domain.UnmatchedStatus
	Path        string
}

// UnmatchedFileRepository persists files awaiting manual resolution.
type UnmatchedFileRepository interface {
	List(ctx context.Context, f UnmatchedFilter) ([]*domain.UnmatchedFile, error)
	Get(ctx context.Context, id string) (*domain.UnmatchedFile, error)
	Save(ctx context.Context, f *domain.UnmatchedFile) error
	Delete(ctx context.Context, id string) error
}

// NotificationDispatcher emits scan lifecycle events to configured backends.
// The no-op adapter logs via slog. Future adapters target Discord, Slack, Gotify, etc.
// The scan service always calls Dispatch — it never checks which adapters are registered.
type NotificationDispatcher interface {
	Dispatch(ctx context.Context, event domain.NotificationEvent) error
}

// ThumbnailCache downloads and stores remote images for offline display in the
// unmatched queue. Store returns the local absolute path of the cached image,
// or "" on any error (errors are logged by the implementation).
type ThumbnailCache interface {
	Store(ctx context.Context, url, key string) string
}
