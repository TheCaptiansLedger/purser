package badger

import (
	"encoding/json"
	"errors"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"sort"
	"strings"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// normalizeSearchText lowercases s and converts typographic apostrophes/quotes
// to their ASCII equivalents so that MusicBrainz-sourced titles (U+2019) match
// against file-tag search terms (U+0027).
func normalizeSearchText(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "’", "'")  // RIGHT SINGLE QUOTATION MARK → apostrophe
	s = strings.ReplaceAll(s, "‘", "'")  // LEFT SINGLE QUOTATION MARK → apostrophe
	s = strings.ReplaceAll(s, "“", "\"") // LEFT DOUBLE QUOTATION MARK → quote
	s = strings.ReplaceAll(s, "”", "\"") // RIGHT DOUBLE QUOTATION MARK → quote
	return s
}

// ── ID / time helpers ─────────────────────────────────────────────────────────

func newID() string { return uuid.New().String() }

func nowStr() string { return time.Now().UTC().Format(time.RFC3339) }

func timeToStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func strToTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func dateToStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func strToDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// ── Key builders ──────────────────────────────────────────────────────────────

func kLE(id string) []byte   { return []byte("le:" + id) }
func kGRP(id string) []byte  { return []byte("grp:" + id) }
func kITM(id string) []byte  { return []byte("itm:" + id) }
func kPER(id string) []byte  { return []byte("per:" + id) }
func kTAG(id string) []byte  { return []byte("tag:" + id) }
func kMF(id string) []byte   { return []byte("mf:" + id) }
func kCFG(key string) []byte { return []byte("cfg:" + key) }
func kSchema() []byte        { return []byte("__schema") }

// External ID reverse index: eid:{entityType}:{source}:{value} → entity ID
func kEID(entityType, source, value string) []byte {
	return []byte("eid:" + entityType + ":" + source + ":" + value)
}

// Entry tag junction: et:{entryID}:{tagID}
func kET(entryID, tagID string) []byte { return []byte("et:" + entryID + ":" + tagID) }

// Tag-entry reverse: tge:{tagID}:{entryID}
func kTGE(tagID, entryID string) []byte { return []byte("tge:" + tagID + ":" + entryID) }

// Group tag junction: gt:{groupID}:{tagID}
func kGT(groupID, tagID string) []byte { return []byte("gt:" + groupID + ":" + tagID) }

// Tag-group reverse: tgg:{tagID}:{groupID}
func kTGG(tagID, groupID string) []byte { return []byte("tgg:" + tagID + ":" + groupID) }

// Item tag junction: it:{itemID}:{tagID}
func kIT(itemID, tagID string) []byte { return []byte("it:" + itemID + ":" + tagID) }

// Tag-item reverse: tgi:{tagID}:{itemID}
func kTGI(tagID, itemID string) []byte { return []byte("tgi:" + tagID + ":" + itemID) }

// Entry-person junction: ep:{entryID}:{personID}:{role}
func kEP(entryID, personID, role string) []byte {
	return []byte("ep:" + entryID + ":" + personID + ":" + role)
}

// Person-entry reverse: epp:{personID}:{entryID}:{role}
func kEPP(personID, entryID, role string) []byte {
	return []byte("epp:" + personID + ":" + entryID + ":" + role)
}

// Item-person junction: ip:{itemID}:{personID}:{role}
func kIP(itemID, personID, role string) []byte {
	return []byte("ip:" + itemID + ":" + personID + ":" + role)
}

// Person-item reverse: ipp:{personID}:{itemID}:{role}
func kIPP(personID, itemID, role string) []byte {
	return []byte("ipp:" + personID + ":" + itemID + ":" + role)
}

// Person role index: pri:{role}:{personID}
func kPRI(role, personID string) []byte { return []byte("pri:" + role + ":" + personID) }

// Media file indexes
func kMFH(hash string) []byte   { return []byte("mfh:" + hash) }
func kMFI(itemID string) []byte { return []byte("mfi:" + itemID) }
func kMFP(path string) []byte   { return []byte("mfp:" + path) }

// Unmatched file records
func kUMF(id string) []byte { return []byte("umf:" + id) }
func pfxUMF() []byte        { return []byte("umf:") }

// Prefixes for iterator scans
func pfxLE() []byte             { return []byte("le:") }
func pfxGRP() []byte            { return []byte("grp:") }
func pfxITM() []byte            { return []byte("itm:") }
func pfxPER() []byte            { return []byte("per:") }
func pfxTAG() []byte            { return []byte("tag:") }
func pfxET(id string) []byte    { return []byte("et:" + id + ":") }
func pfxTGE(id string) []byte   { return []byte("tge:" + id + ":") }
func pfxGT(id string) []byte    { return []byte("gt:" + id + ":") }
func pfxTGG(id string) []byte   { return []byte("tgg:" + id + ":") }
func pfxIT(id string) []byte    { return []byte("it:" + id + ":") }
func pfxTGI(id string) []byte   { return []byte("tgi:" + id + ":") }
func pfxEP(id string) []byte    { return []byte("ep:" + id + ":") }
func pfxEPP(id string) []byte   { return []byte("epp:" + id + ":") }
func pfxIP(id string) []byte    { return []byte("ip:" + id + ":") }
func pfxIPP(id string) []byte   { return []byte("ipp:" + id + ":") }
func pfxPRI(role string) []byte { return []byte("pri:" + role + ":") }

// ── Storage structs ───────────────────────────────────────────────────────────

type extIDRecord struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

func toExtIDRecords(ids []domain.ExternalID) []extIDRecord {
	if len(ids) == 0 {
		return nil
	}
	out := make([]extIDRecord, len(ids))
	for i, id := range ids {
		out[i] = extIDRecord{Source: string(id.Source), Value: id.Value}
	}
	return out
}

func fromExtIDRecords(recs []extIDRecord) []domain.ExternalID {
	if len(recs) == 0 {
		return nil
	}
	out := make([]domain.ExternalID, len(recs))
	for i, r := range recs {
		out[i] = domain.ExternalID{Source: domain.ExternalIDSource(r.Source), Value: r.Value}
	}
	return out
}

type libEntryRecord struct {
	ID                string         `json:"id"`
	ContentType       string         `json:"content_type"`
	Kind              string         `json:"kind"`
	Name              string         `json:"name"`
	SortName          string         `json:"sort_name"`
	SortKey           string         `json:"sort_key,omitempty"`
	Overview          string         `json:"overview,omitempty"`
	ParentID          string         `json:"parent_id,omitempty"`
	Monitored         bool           `json:"monitored"`
	MonitorMode       string         `json:"monitor_mode"`
	Status            string         `json:"status"`
	QualityProfileID  string         `json:"quality_profile_id,omitempty"`
	MetadataProfileID string         `json:"metadata_profile_id,omitempty"`
	Path              string         `json:"path,omitempty"`
	ImagePath         string         `json:"image_path,omitempty"`
	BannerURL         *string        `json:"banner_url,omitempty"`
	ExternalIDs       []extIDRecord  `json:"external_ids,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	LockedFields      []string       `json:"locked_fields,omitempty"`
	AddedAt           string         `json:"added_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type groupRecord struct {
	ID             string         `json:"id"`
	LibraryEntryID string         `json:"library_entry_id"`
	ContentType    string         `json:"content_type"` // denormalized from parent entry
	Title          string         `json:"title"`
	SortName       string         `json:"sort_name,omitempty"`
	SortKey        string         `json:"sort_key,omitempty"`
	Number         int            `json:"number"`
	Year           int            `json:"year,omitempty"`
	Overview       string         `json:"overview,omitempty"`
	Monitored      bool           `json:"monitored"`
	MonitorMode    string         `json:"monitor_mode"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	LockedFields   []string       `json:"locked_fields,omitempty"`
	CoverPath      string         `json:"cover_path,omitempty"`
	ExternalIDs    []extIDRecord  `json:"external_ids,omitempty"`
}

type itemPersonRecord struct {
	PersonID string `json:"person_id"`
	Role     string `json:"role"`
}

type itemTagRecord struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

type itemRecord struct {
	ID             string             `json:"id"`
	ContentType    string             `json:"content_type"`
	LibraryEntryID string             `json:"library_entry_id"`
	GroupID        string             `json:"group_id,omitempty"`
	Title          string             `json:"title"`
	Overview       string             `json:"overview,omitempty"`
	Date           string             `json:"date,omitempty"`
	Sequence       string             `json:"sequence,omitempty"`
	SortKey        string             `json:"sort_key,omitempty"`
	RuntimeSeconds int                `json:"runtime_seconds,omitempty"`
	Monitored      bool               `json:"monitored"`
	Status         string             `json:"status"`
	CoverPath      string             `json:"cover_path,omitempty"`
	Metadata       map[string]any     `json:"metadata,omitempty"`
	LockedFields   []string           `json:"locked_fields,omitempty"`
	ExternalIDs    []extIDRecord      `json:"external_ids,omitempty"`
	Tags           []itemTagRecord    `json:"tags,omitempty"`
	People         []itemPersonRecord `json:"people,omitempty"`
	AddedAt        string             `json:"added_at"`
	UpdatedAt      string             `json:"updated_at"`
}

type personRecord struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	SortName     string         `json:"sort_name,omitempty"`
	SortKey      string         `json:"sort_key,omitempty"`
	Overview     string         `json:"overview,omitempty"`
	Monitored    bool           `json:"monitored"`
	MonitorMode  string         `json:"monitor_mode"`
	ImagePath    string         `json:"image_path,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	LockedFields []string       `json:"locked_fields,omitempty"`
	Aliases      []string       `json:"aliases,omitempty"`
	Roles        []string       `json:"roles,omitempty"`
	ExternalIDs  []extIDRecord  `json:"external_ids,omitempty"`
	AddedAt      string         `json:"added_at"`
}

type tagRecord struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Scope   string `json:"scope"`
	SortKey string `json:"sort_key,omitempty"`
}

type mediaFileRecord struct {
	ID              string `json:"id"`
	ItemID          string `json:"item_id"`
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	OSHash          string `json:"oshash,omitempty"`
	MD5             string `json:"md5,omitempty"`
	Quality         string `json:"quality,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	Codec           string `json:"codec,omitempty"`
	Container       string `json:"container,omitempty"`
	MatchConfidence string `json:"match_confidence,omitempty"`
	AddedAt         string `json:"added_at"`
}

type fingerprintRecord struct {
	OSHash       string            `json:"oshash,omitempty"`
	PHash        string            `json:"phash,omitempty"`
	AcoustID     string            `json:"acoustid,omitempty"`
	EmbeddedTags map[string]string `json:"embedded_tags,omitempty"`
	ISBN         string            `json:"isbn,omitempty"`
}

type matchCandidateRecord struct {
	ItemID       string               `json:"item_id,omitempty"`
	ExternalItem *domain.ExternalItem `json:"external_item,omitempty"`
	Confidence   float64              `json:"confidence"`
	Source       string               `json:"source"`
}

type unmatchedFileRecord struct {
	ID            string                 `json:"id"`
	Path          string                 `json:"path"`
	Size          int64                  `json:"size"`
	ContentType   string                 `json:"content_type"`
	Fingerprint   *fingerprintRecord     `json:"fingerprint,omitempty"`
	Candidates    []matchCandidateRecord `json:"candidates,omitempty"`
	Status        string                 `json:"status"`
	DiscoveredAt  string                 `json:"discovered_at"`
	DuplicateOf   string                 `json:"duplicate_of,omitempty"`
	ThumbnailPath string                 `json:"thumbnail_path,omitempty"`
}

type entryPersonJunction struct {
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// ── BadgerDB transaction helpers ──────────────────────────────────────────────

// getJSON reads a key from a transaction and JSON-unmarshals it into dst.
// Returns errs.ErrNotFound when the key does not exist so callers can use
// errs.IsNotFound without knowing the underlying storage engine's error type.
func getJSON[T any](txn *badgerdb.Txn, key []byte) (*T, error) {
	item, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	var rec T
	if err := item.Value(func(val []byte) error {
		return json.Unmarshal(val, &rec)
	}); err != nil {
		return nil, err
	}
	return &rec, nil
}

// setJSON JSON-marshals v and sets key in the transaction.
func setJSON(txn *badgerdb.Txn, key []byte, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return txn.Set(key, data)
}

// iterPrefix calls fn for each key with the given prefix (keys-only scan).
// fn receives a copy of the full key; returning false stops iteration.
func iterPrefix(txn *badgerdb.Txn, prefix []byte, fn func(key []byte) bool) {
	opts := badgerdb.DefaultIteratorOptions
	opts.Prefix = prefix
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		if !fn(it.Item().KeyCopy(nil)) {
			break
		}
	}
}

// iterPrefixValues calls fn for each key+value with the given prefix.
// fn receives copies of key and value; returning false stops iteration.
func iterPrefixValues(txn *badgerdb.Txn, prefix []byte, fn func(key, val []byte) bool) {
	opts := badgerdb.DefaultIteratorOptions
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		k := it.Item().KeyCopy(nil)
		var cont bool
		_ = it.Item().Value(func(v []byte) error {
			cp := make([]byte, len(v))
			copy(cp, v)
			cont = fn(k, cp)
			return nil
		})
		if !cont {
			break
		}
	}
}

// deletePrefix deletes all keys with the given prefix within txn.
func deletePrefix(txn *badgerdb.Txn, prefix []byte) error {
	var keys [][]byte
	iterPrefix(txn, prefix, func(key []byte) bool {
		keys = append(keys, key)
		return true
	})
	for _, k := range keys {
		if err := txn.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

// suffixAfter returns the portion of key after prefix.
func suffixAfter(key, prefix []byte) string {
	if len(key) <= len(prefix) {
		return ""
	}
	return string(key[len(prefix):])
}

// splitN splits s on ":" into at most n parts (last part may contain ":").
func splitN(s string, n int) []string {
	if n <= 0 || s == "" {
		return nil
	}
	parts := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := -1
		for j := 0; j < len(s); j++ {
			if s[j] == ':' {
				idx = j
				break
			}
		}
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+1:]
	}
	parts = append(parts, s)
	return parts
}

// ── Tag helpers shared across repos ──────────────────────────────────────────

func loadTagByID(txn *badgerdb.Txn, tagID string) (*domain.Tag, error) {
	rec, err := getJSON[tagRecord](txn, kTAG(tagID))
	if err != nil {
		return nil, err
	}
	return tagFromRecord(rec), nil
}

func tagFromRecord(r *tagRecord) *domain.Tag {
	sortKey := r.SortKey
	if sortKey == "" {
		sortKey = domain.TagSortKey(r.Key, r.Value)
	}
	return &domain.Tag{
		ID:      r.ID,
		Key:     domain.TagKey(r.Key),
		Value:   r.Value,
		Scope:   domain.TagScope(r.Scope),
		SortKey: sortKey,
	}
}

// loadTagsForPrefix scans junction keys under prefix, parses the tagID from
// each key suffix, and loads the corresponding tag record.
func loadTagsForPrefix(txn *badgerdb.Txn, prefix []byte) ([]domain.Tag, error) {
	var tagIDs []string
	iterPrefix(txn, prefix, func(key []byte) bool {
		tagIDs = append(tagIDs, suffixAfter(key, prefix))
		return true
	})
	tags := make([]domain.Tag, 0, len(tagIDs))
	for _, id := range tagIDs {
		t, err := loadTagByID(txn, id)
		if err != nil {
			continue // tag deleted concurrently; skip
		}
		tags = append(tags, *t)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Value < tags[j].Value })
	return tags, nil
}

// collectIDsForTagFilter returns the set of entity IDs that carry a tag matching
// key and/or value, using the reverse-index prefix builder supplied by the caller
// (pfxTGE for library entries, pfxTGG for groups).
func collectIDsForTagFilter(txn *badgerdb.Txn, key, value string, pfxFn func(string) []byte) map[string]struct{} {
	var matchTagIDs []string
	iterPrefixValues(txn, pfxTAG(), func(_, val []byte) bool {
		var rec tagRecord
		if json.Unmarshal(val, &rec) == nil {
			keyOK := key == "" || rec.Key == key
			valOK := value == "" || rec.Value == value
			if keyOK && valOK {
				matchTagIDs = append(matchTagIDs, rec.ID)
			}
		}
		return true
	})

	ids := make(map[string]struct{})
	for _, tagID := range matchTagIDs {
		prefix := pfxFn(tagID)
		iterPrefix(txn, prefix, func(k []byte) bool {
			ids[suffixAfter(k, prefix)] = struct{}{}
			return true
		})
	}
	return ids
}

// ── Person stub loader ────────────────────────────────────────────────────────

// loadPersonStub loads the minimal Person fields (Name, SortName, ImagePath)
// needed when embedding a person reference in EntryPerson/ItemPerson.
func loadPersonStub(txn *badgerdb.Txn, personID string) *domain.Person {
	rec, err := getJSON[personRecord](txn, kPER(personID))
	if err != nil {
		return nil
	}
	return &domain.Person{
		ID:        rec.ID,
		Name:      rec.Name,
		SortName:  rec.SortName,
		ImagePath: rec.ImagePath,
	}
}
