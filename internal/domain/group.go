package domain

// Group is an intermediate content grouping within a LibraryEntry:
// Season (TV), Album (music), or an optional named series within an adult/JAV studio.
type Group struct {
	ID             string
	LibraryEntryID string
	Title          string
	SortName       string
	Number         int // season number, album number, etc.
	Year           int
	Overview       string
	Monitored      bool
	MonitorMode    MonitorMode
	ExternalIDs    []ExternalID
	Metadata       map[string]any
	LockedFields   []string
	CoverPath      string
	SortKey        string
	Tags           []Tag
}

// ApplyDefaults computes the SortKey before persistence.
func (g *Group) ApplyDefaults() {
	g.SortKey = GroupSortKey(g.Number, g.Title)
}
