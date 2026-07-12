package domain

// LibraryEntry is the top-level, independently monitorable unit — an
// Artist, a Studio, a Series, a Movie, an Author. Self-referential via
// ParentID (Network -> Studio) for hierarchy that genuinely nests;
// Collection (collection.go) is used instead when entries need to be
// grouped without losing their own independent identity, e.g. a movie in a
// franchise still has its own monitoring/path/status like a standalone
// movie would.
type LibraryEntry struct {
	ID          string      `validate:"required"`
	ContentType ContentType `validate:"required"`
	Kind        Kind        `validate:"required"`
	Name        string      `validate:"required"`
	SortName    string
	Overview    string
	ParentID    string

	Monitored   bool
	MonitorMode MonitorMode `validate:"required,oneof=all future none latest"`
	Status      string

	QualityProfileID  string
	MetadataProfileID string
	Path              string

	Metadata map[string]any
}

// Validate checks LibraryEntry's invariants: ID, ContentType, Kind, and
// Name are required, and MonitorMode must be one of the known monitoring
// modes.
func (l *LibraryEntry) Validate() error {
	return validateStruct(l)
}
