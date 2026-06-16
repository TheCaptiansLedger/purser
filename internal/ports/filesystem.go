package ports

import (
	"context"
	"time"
)

// FileInfo describes a file or directory on disk.
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// FileSystem is the port for filesystem operations used during import and library organization.
type FileSystem interface {
	Stat(ctx context.Context, path string) (*FileInfo, error)
	Move(ctx context.Context, src, dst string) error
	OSHash(ctx context.Context, path string) (string, error)
	Walk(ctx context.Context, root string, fn func(FileInfo) error) error
}
