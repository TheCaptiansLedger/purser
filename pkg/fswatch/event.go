package fswatch

import "time"

// EventKind describes a unit's lifecycle transition, not the raw fs
// operation that triggered it — a directory's first settled activity is
// Created even though internally it was assembled from several raw
// create/write events.
type EventKind uint8

// The possible EventKind values.
const (
	Created EventKind = iota
	Updated
	Removed
)

// String implements fmt.Stringer so EventKind reads sensibly in log
// attributes and metric labels.
func (k EventKind) String() string {
	switch k {
	case Created:
		return "created"
	case Updated:
		return "updated"
	case Removed:
		return "removed"
	default:
		return "unknown"
	}
}

// Event is a single, debounced notification about a unit — a directory
// (or, depending on the UnitResolver, a single file) whose subtree has
// been quiet for at least Config.SettleWindow, or reached Config.MaxWait.
type Event struct {
	// Root is the watched root this unit falls under.
	Root string

	// Path is the resolved unit path (see UnitResolver) — the directory
	// (or file) the caller should treat as the thing that changed, not
	// necessarily the exact path any individual raw event reported.
	Path string

	Kind EventKind

	// Files lists the paths that changed within Path's subtree during
	// this settle window, deduplicated and sorted. Empty for Removed
	// events.
	Files []string

	Time time.Time
}
