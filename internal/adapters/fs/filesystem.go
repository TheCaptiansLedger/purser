package fs

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"purser/internal/ports"
)

type osFileSystem struct{}

// NewFileSystem returns a ports.FileSystem backed by the real OS.
func NewFileSystem() ports.FileSystem {
	return &osFileSystem{}
}

var _ ports.FileSystem = (*osFileSystem)(nil)

func (f *osFileSystem) Stat(_ context.Context, path string) (*ports.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return &ports.FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime().UTC(),
		IsDir:   info.IsDir(),
	}, nil
}

func (f *osFileSystem) Move(_ context.Context, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}
	return os.Rename(src, dst)
}

func (f *osFileSystem) OSHash(_ context.Context, path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // path comes from library scan
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	const chunkSize = 64 * 1024
	fileSize := info.Size()
	if fileSize == 0 {
		return "", nil
	}
	hash := uint64(fileSize) //nolint:gosec // intentional size-to-hash cast

	buf := make([]byte, chunkSize)
	n, _ := io.ReadFull(file, buf)
	for i := 0; i+8 <= n; i += 8 {
		hash += binary.LittleEndian.Uint64(buf[i : i+8])
	}
	if fileSize >= chunkSize {
		if _, err := file.Seek(-chunkSize, io.SeekEnd); err != nil {
			return "", fmt.Errorf("seek %s: %w", path, err)
		}
		n, _ = io.ReadFull(file, buf)
		for i := 0; i+8 <= n; i += 8 {
			hash += binary.LittleEndian.Uint64(buf[i : i+8])
		}
	}
	return fmt.Sprintf("%016x", hash), nil
}

func (f *osFileSystem) Walk(_ context.Context, root string, fn func(ports.FileInfo) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr
		}
		return fn(ports.FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
			IsDir:   d.IsDir(),
		})
	})
}
