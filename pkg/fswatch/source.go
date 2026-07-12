package fswatch

// RawOp identifies the kind of raw filesystem operation a Source observed.
type RawOp uint8

// The possible RawOp values.
const (
	OpCreate RawOp = iota
	OpWrite
	OpRemove
	OpRename
)

// String implements fmt.Stringer so RawOp reads sensibly in log attributes.
func (op RawOp) String() string {
	switch op {
	case OpCreate:
		return "create"
	case OpWrite:
		return "write"
	case OpRemove:
		return "remove"
	case OpRename:
		return "rename"
	default:
		return "unknown"
	}
}

// RawEvent is a single, low-level filesystem notification from a Source,
// scoped to exactly one path.
type RawEvent struct {
	Path string
	Op   RawOp
}

// Source is the port a low-level filesystem notification backend
// implements. Watcher composes a Source with recursive directory
// management and debounced coalescing (see watcher.go); a Source itself
// only reports raw, per-path events for paths it has been told to watch —
// it has no concept of "settling" or of a directory subtree.
//
// A Source is driven by exactly one goroutine: Add, Remove, and draining
// Events/Errors all happen from Watcher's single internal run loop. Close
// may be called from a different goroutine (Watcher.Close), but only after
// that run loop has fully stopped, so implementations do not need to
// support Close running concurrently with Add/Remove/drain.
type Source interface {
	// Add starts watching path (a single directory or file, non-recursive
	// — recursion is Watcher's responsibility).
	Add(path string) error

	// Remove stops watching path. Removing a path that isn't watched is
	// not an error.
	Remove(path string) error

	// Events returns the channel of raw filesystem notifications for
	// every currently watched path. The channel is closed after Close.
	Events() <-chan RawEvent

	// Errors returns the channel of asynchronous errors encountered
	// while watching (e.g. a watched directory removed out from under
	// the watch). The channel is closed after Close.
	Errors() <-chan error

	// Close releases all resources held by the Source.
	Close() error
}
