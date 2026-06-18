# Key Ports (Interfaces)

All ports live in `internal/ports/`. Adapters implement these interfaces; app services depend only on them.

```go
// Metadata sources — StashDB, TPDB, TMDB, TVDB, MusicBrainz, etc.
type MetadataSource interface {
    Name() string
    ContentTypes() []string
    FindByExternalID(ctx context.Context, id string) (*domain.ExternalItem, error)
    FindByHash(ctx context.Context, hash string) (*domain.ExternalItem, error)
    SearchByTitle(ctx context.Context, title string, limit int) ([]*domain.ExternalItem, error)
    FetchEntryContent(ctx context.Context, entryID string) ([]*domain.ExternalItem, error)
}

// Indexers — Prowlarr
type Indexer interface {
    Search(ctx context.Context, query string, cats []Category) ([]Release, error)
}

// Download clients — qBittorrent, Transmission, etc.
type DownloadClient interface {
    Add(ctx context.Context, r Release) (jobID string, err error)
    Status(ctx context.Context, jobID string) (DownloadStatus, error)
    Remove(ctx context.Context, jobID string, deleteFiles bool) error
}

// Repositories — one per aggregate root, swappable (SQLite ↔ PostgreSQL)
type LibraryEntryRepository interface {
    Get(ctx context.Context, id string) (*domain.LibraryEntry, error)
    List(ctx context.Context, filter LibraryFilter) ([]*domain.LibraryEntry, error)
    Save(ctx context.Context, e *domain.LibraryEntry) error
    Delete(ctx context.Context, id string) error
    GetPeople(ctx context.Context, entryID string) ([]domain.EntryPerson, error)
    SavePerson(ctx context.Context, entryID string, ep domain.EntryPerson) error
    RemovePerson(ctx context.Context, entryID, personID, role string) error
}

// Separate repository interfaces for:
// GroupRepository, ItemRepository, PersonRepository,
// ReleaseRepository, DownloadRepository

// Background jobs
type JobQueue interface {
    Enqueue(ctx context.Context, job *domain.Job) error
    Cancel(ctx context.Context, jobID string) error
}

// Filesystem
type FileSystem interface {
    Stat(ctx context.Context, path string) (*FileInfo, error)
    Move(ctx context.Context, src, dst string) error
    OSHash(ctx context.Context, path string) (string, error)
}
```
