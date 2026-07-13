package config

// Media configures where Purser stores image blobs
// (internal/adapters/imagestore/local) — see
// docs/adr/0013-image-blob-storage.md.
type Media struct {
	// Path is the root directory image blobs are written under. Left
	// empty here means "derive from Paths.DataDir" — see Config.Load.
	Path string `mapstructure:"path"`
}

// DefaultMedia returns Media's default: Path left empty so
// Config.DefaultConfig can derive it from Paths.DataDir.
func DefaultMedia() Media {
	return Media{}
}
