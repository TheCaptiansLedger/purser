package ports

import "context"

// DiscoveredFile is one regular file a FileWalker found under a root.
type DiscoveredFile struct {
	Path string
	Size int64
}

// FileWalker recursively enumerates every regular file under root — the
// discovery step of the common scan pipeline (docs/adr/0024-pipeline-core.md).
// Deliberately minimal for this issue's scope: no dedup, no ignore-glob
// filtering (both later sub-issues). Directories, symlinks, and other
// non-regular files (devices, sockets, etc.) are never included in the
// result.
type FileWalker interface {
	Walk(ctx context.Context, root string) ([]DiscoveredFile, error)
}
