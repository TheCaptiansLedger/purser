package config

// Sources groups configuration for external metadata/image providers under
// the "sources" key, matching the PURSER_SOURCES_<PROVIDER>_<KEY> env var
// convention already established in .env.example and cited directly by
// docs/adr/0010-configuration.md's own context section (its example is
// literally PURSER_SOURCES_STASHDB_API_KEY). StashDB and ThePornDB are
// wired here — .env.example also documents TMDb/TVDb/OMDb/fanart.tv/
// TheAudioDB/Last.fm keys under the same "sources" prefix, but none of
// those have an adapter yet, so their config structs don't exist yet
// either.
//
// ThePornDB's field key is "tpdb" (Config.Sources.ThePornDB, mapstructure
// "tpdb"), not "theporndb" — matching .env.example's already-documented
// PURSER_SOURCES_TPDB_ENABLED/PURSER_SOURCES_TPDB_API_KEY convention
// (confirmed present there, distinct from the legacy unprefixed
// THEPORNDB_API_KEY .env still carries — see
// docs/technical/afterdark-data_model.md Section 1's note on that gap).
// Go identifiers stay the full "ThePornDB" for readability; only the wire
// key is abbreviated, same asymmetry StashDB doesn't need since its short
// and long names already coincide.
//
// MusicBrainz and AcoustID predate this struct and still live as flat
// top-level Config fields (musicbrainz.*, acoustid.*) rather than under
// sources.* — a real, pre-existing deviation from this same ADR-0010
// example (.env.example even documents PURSER_SOURCES_MUSICBRAINZ_ENABLED
// and PURSER_SOURCES_ACOUSTID_* keys that current Load() never reads).
// Left alone here deliberately: reconciling it means moving two existing,
// already-wired adapters' config and is out of scope for adding StashDB/
// ThePornDB.
type Sources struct {
	StashDB   StashDB   `mapstructure:"stashdb"`
	ThePornDB ThePornDB `mapstructure:"tpdb"`
}

// DefaultSources returns Sources' defaults.
func DefaultSources() Sources {
	return Sources{StashDB: DefaultStashDB(), ThePornDB: DefaultThePornDB()}
}
