package media

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// ImagePath returns the on-disk path for an entity's image.
// Layout: {base}/{entityType}/{id[0:2]}/{id}{ext}
func ImagePath(base, entityType, id, ext string) string {
	return filepath.Join(base, entityType, shard(id), id+ext)
}

// EnsureDirs creates the top-level entity subdirectories under base.
// Shard directories are created lazily on first write.
func EnsureDirs(base string) error {
	for _, sub := range []string{"entries", "items", "people"} {
		if err := os.MkdirAll(filepath.Join(base, sub), 0o755); err != nil {
			return fmt.Errorf("create media dir %s: %w", sub, err)
		}
	}
	return nil
}

// MigrateFlat moves image files from the old flat layout ({base}/{type}/{id+ext})
// to the sharded layout ({base}/{type}/{id[0:2]}/{id+ext}).
// Files already inside a subdirectory are skipped.
func MigrateFlat(base string) error {
	for _, entityType := range []string{"entries", "items", "people"} {
		dir := filepath.Join(base, entityType)
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			id := name[:len(name)-len(ext)]
			if len(id) < 2 {
				continue
			}
			shardDir := filepath.Join(dir, shard(id))
			if err := os.MkdirAll(shardDir, 0o755); err != nil {
				return fmt.Errorf("create shard dir %s: %w", shardDir, err)
			}
			src := filepath.Join(dir, name)
			dst := filepath.Join(shardDir, name)
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("migrate %s: %w", name, err)
			}
			slog.Debug("migrated image", "src", src, "dst", dst)
		}
	}
	return nil
}

func shard(id string) string {
	if len(id) >= 2 {
		return id[:2]
	}
	return id
}
