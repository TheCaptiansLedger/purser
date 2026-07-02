package ports

import (
	"context"
	"io"
)

// StorageAdminPort provides administrative operations over the storage backend.
type StorageAdminPort interface {
	DriverName() string
	BackupMeta() (contentType, filename string)
	Stats(ctx context.Context) (*StorageStats, error)
	Backup(ctx context.Context, w io.Writer) error
	Restore(ctx context.Context, r io.Reader, shutdownFn func()) (*StorageStats, error)
}

// StorageStats describes the current state of the storage backend.
type StorageStats struct {
	Driver        string            `json:"driver"`
	DriverVersion string            `json:"driver_version"`
	SizeBytes     int64             `json:"size_bytes"`
	Collections   []CollectionStats `json:"collections"`
	Extra         map[string]any    `json:"extra,omitempty"`
}

// CollectionStats describes a single named collection (table) within the backend.
type CollectionStats struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
