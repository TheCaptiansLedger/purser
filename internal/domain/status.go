package domain

// ItemStatus tracks the acquisition state of a leaf item.
type ItemStatus string

const (
	StatusWanted      ItemStatus = "wanted"
	StatusGrabbed     ItemStatus = "grabbed"
	StatusDownloading ItemStatus = "downloading"
	StatusImported    ItemStatus = "imported"
	StatusMissing     ItemStatus = "missing"
	StatusSkipped     ItemStatus = "skipped"
)
