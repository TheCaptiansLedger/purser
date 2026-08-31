package config

// Sources groups configuration for external metadata/image providers under
// the "sources" key, matching the PURSER_SOURCES_<PROVIDER>_<KEY> env var
// convention already established in .env.example and cited directly by
// docs/adr/0010-configuration.md's own context section (its example is
// literally PURSER_SOURCES_STASHDB_API_KEY). StashDB, ThePornDB,
// TheAudioDB, and FanartTV are wired here — .env.example also documents
// TMDb/TVDb/OMDb/Last.fm keys under the same "sources" prefix, but none of
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
// and long names already coincide. FanartTV's field key is "fanart", same
// asymmetry for the same reason — matching .env.example's
// PURSER_SOURCES_FANART_ENABLED/PURSER_SOURCES_FANART_API_KEY convention
// (added alongside this adapter — .env.example only documented TheAudioDB's
// section before this, despite this doc comment's own prior claim
// otherwise; a real, now-fixed gap). TheAudioDB's field key is
// "theaudiodb", no asymmetry needed since its short and long names already
// coincide, same as StashDB.
//
// MusicBrainz and AcoustID predate this struct and still live as flat
// top-level Config fields (musicbrainz.*, acoustid.*) rather than under
// sources.* — a real, pre-existing deviation from this same ADR-0010
// example (.env.example even documents PURSER_SOURCES_MUSICBRAINZ_ENABLED
// and PURSER_SOURCES_ACOUSTID_* keys that current Load() never reads).
// Left alone here deliberately: reconciling it means moving two existing,
// already-wired adapters' config and is out of scope for adding StashDB/
// ThePornDB/TheAudioDB/FanartTV.
type Sources struct {
	StashDB    StashDB    `mapstructure:"stashdb"`
	ThePornDB  ThePornDB  `mapstructure:"tpdb"`
	TheAudioDB TheAudioDB `mapstructure:"theaudiodb"`
	FanartTV   FanartTV   `mapstructure:"fanart"`
	Wikidata   Wikidata   `mapstructure:"wikidata"`
}

// DefaultSources returns Sources' defaults.
func DefaultSources() Sources {
	return Sources{
		StashDB:    DefaultStashDB(),
		ThePornDB:  DefaultThePornDB(),
		TheAudioDB: DefaultTheAudioDB(),
		FanartTV:   DefaultFanartTV(),
		Wikidata:   DefaultWikidata(),
	}
}
