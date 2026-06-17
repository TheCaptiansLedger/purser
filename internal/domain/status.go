package domain

// ItemStatus tracks the acquisition state of a leaf item.
type ItemStatus string

// Item status constants covering the full acquisition state machine.
const (
	StatusWanted      ItemStatus = "wanted"
	StatusGrabbed     ItemStatus = "grabbed"
	StatusDownloading ItemStatus = "downloading"
	StatusImported    ItemStatus = "imported"
	StatusMissing     ItemStatus = "missing"
	StatusSkipped     ItemStatus = "skipped"
)
