package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
	"strings"

	badgerdb "github.com/dgraph-io/badger/v4"
)

type libraryEntryRepo struct {
	db *badgerdb.DB
}

// NewLibraryEntryRepo returns a LibraryEntryRepository backed by BadgerDB.
func NewLibraryEntryRepo(db *badgerdb.DB) ports.LibraryEntryRepository {
	return &libraryEntryRepo{db: db}
}

var _ ports.LibraryEntryRepository = (*libraryEntryRepo)(nil)

func (r *libraryEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	var e *domain.LibraryEntry
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var err error
		e, err = loadLibraryEntry(txn, id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get library entry %s: %w", id, err)
	}
	return e, nil
}

func loadLibraryEntry(txn *badgerdb.Txn, id string) (*domain.LibraryEntry, error) {
	rec, err := getJSON[libEntryRecord](txn, kLE(id))
	if err != nil {
		return nil, err
	}
	e := entryFromRecord(rec)

	tags, err := loadTagsForPrefix(txn, pfxET(id))
	if err != nil {
		return nil, err
	}
	e.Tags = tags

	people, err := loadEntryPeople(txn, id)
	if err != nil {
		return nil, err
	}
	e.People = people

	return e, nil
}

func entryFromRecord(rec *libEntryRecord) *domain.LibraryEntry {
	sortKey := rec.SortKey
	if sortKey == "" {
		sortKey = domain.NameSortKey(rec.SortName, rec.Name)
	}
	return &domain.LibraryEntry{
		ID:                rec.ID,
		ContentType:       domain.ContentType(rec.ContentType),
		Kind:              domain.Kind(rec.Kind),
		Name:              rec.Name,
		SortName:          rec.SortName,
		SortKey:           sortKey,
		Overview:          rec.Overview,
		ParentID:          rec.ParentID,
		Monitored:         rec.Monitored,
		MonitorMode:       domain.MonitorMode(rec.MonitorMode),
		Status:            domain.EntryStatus(rec.Status),
		QualityProfileID:  rec.QualityProfileID,
		MetadataProfileID: rec.MetadataProfileID,
		Path:              rec.Path,
		ImagePath:         rec.ImagePath,
		BannerURL:         rec.BannerURL,
		ExternalIDs:       fromExtIDRecords(rec.ExternalIDs),
		Metadata:          rec.Metadata,
		LockedFields:      rec.LockedFields,
		AddedAt:           strToTime(rec.AddedAt),
		UpdatedAt:         strToTime(rec.UpdatedAt),
	}
}

func loadEntryPeople(txn *badgerdb.Txn, entryID string) ([]domain.EntryPerson, error) {
	var people []domain.EntryPerson
	prefix := pfxEP(entryID)
	iterPrefixValues(txn, prefix, func(key, val []byte) bool {
		parts := splitN(suffixAfter(key, prefix), 2)
		if len(parts) < 2 {
			return true
		}
		personID, role := parts[0], parts[1]

		var junc entryPersonJunction
		_ = json.Unmarshal(val, &junc)

		stub := loadPersonStub(txn, personID)
		people = append(people, domain.EntryPerson{
			PersonID:  personID,
			Role:      role,
			StartDate: strToDate(junc.StartDate),
			EndDate:   strToDate(junc.EndDate),
			Person:    stub,
		})
		return true
	})
	sort.Slice(people, func(i, j int) bool {
		ni, nj := personSortKey(people[i].Person), personSortKey(people[j].Person)
		return ni < nj
	})
	return people, nil
}

func personSortKey(p *domain.Person) string {
	if p == nil {
		return ""
	}
	if p.SortKey != "" {
		return p.SortKey
	}
	return domain.NameSortKey(p.SortName, p.Name)
}

func (r *libraryEntryRepo) List(_ context.Context, f ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	var results []*domain.LibraryEntry

	err := r.db.View(func(txn *badgerdb.Txn) error {
		// For PersonID filter, collect qualifying entry IDs via the epp: reverse index.
		var personEntryIDs map[string]struct{}
		if f.PersonID != "" {
			personEntryIDs = make(map[string]struct{})
			iterPrefix(txn, pfxEPP(f.PersonID), func(key []byte) bool {
				parts := splitN(suffixAfter(key, pfxEPP(f.PersonID)), 2)
				if len(parts) >= 1 {
					personEntryIDs[parts[0]] = struct{}{}
				}
				return true
			})
		}

		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxLE()
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(pfxLE()); it.ValidForPrefix(pfxLE()); it.Next() {
			var rec libEntryRecord
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			}); err != nil {
				continue
			}

			if !matchesEntryFilter(rec, f, personEntryIDs) {
				continue
			}

			results = append(results, entryFromRecord(&rec))
		}
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list library entries: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SortKey < results[j].SortKey
	})

	total := len(results)
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	start := f.Offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return results[start:end], total, nil
}

func matchesEntryFilter(rec libEntryRecord, f ports.LibraryFilter, personEntryIDs map[string]struct{}) bool {
	if f.ContentType != "" && rec.ContentType != string(f.ContentType) {
		return false
	}
	if f.Kind != "" && rec.Kind != string(f.Kind) {
		return false
	}
	if f.ParentID != "" && rec.ParentID != f.ParentID {
		return false
	}
	if f.Monitored != nil && rec.Monitored != *f.Monitored {
		return false
	}
	if f.Search != "" && !strings.Contains(strings.ToLower(rec.Name), strings.ToLower(f.Search)) {
		return false
	}
	if personEntryIDs != nil {
		if _, ok := personEntryIDs[rec.ID]; !ok {
			return false
		}
	}
	return true
}

func (r *libraryEntryRepo) Save(_ context.Context, e *domain.LibraryEntry) error {
	if e.ID == "" {
		e.ID = newID()
	}
	e.ApplyDefaults()
	now := nowStr()
	if e.AddedAt.IsZero() {
		e.AddedAt = strToTime(now)
	}
	e.UpdatedAt = strToTime(now)

	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Clean up old external ID index entries.
		if old, err := getJSON[libEntryRecord](txn, kLE(e.ID)); err == nil {
			for _, eid := range old.ExternalIDs {
				_ = txn.Delete(kEID("library_entry", eid.Source, eid.Value))
			}
		}

		// Clean up old tag junction keys.
		if err := deleteETJunction(txn, e.ID); err != nil {
			return err
		}

		rec := libEntryRecord{
			ID:                e.ID,
			ContentType:       string(e.ContentType),
			Kind:              string(e.Kind),
			Name:              e.Name,
			SortName:          e.SortName,
			SortKey:           e.SortKey,
			Overview:          e.Overview,
			ParentID:          e.ParentID,
			Monitored:         e.Monitored,
			MonitorMode:       string(e.MonitorMode),
			Status:            string(e.Status),
			QualityProfileID:  e.QualityProfileID,
			MetadataProfileID: e.MetadataProfileID,
			Path:              e.Path,
			ImagePath:         e.ImagePath,
			BannerURL:         e.BannerURL,
			ExternalIDs:       toExtIDRecords(e.ExternalIDs),
			Metadata:          e.Metadata,
			LockedFields:      e.LockedFields,
			AddedAt:           timeToStr(e.AddedAt),
			UpdatedAt:         now,
		}
		if err := setJSON(txn, kLE(e.ID), rec); err != nil {
			return err
		}

		// Write new external ID index entries.
		for _, eid := range e.ExternalIDs {
			if err := txn.Set(kEID("library_entry", string(eid.Source), eid.Value), []byte(e.ID)); err != nil {
				return err
			}
		}

		// Write new tag junction keys.
		for _, tag := range e.Tags {
			if err := txn.Set(kET(e.ID, tag.ID), []byte{}); err != nil {
				return err
			}
			if err := txn.Set(kTGE(tag.ID, e.ID), []byte{}); err != nil {
				return err
			}
		}

		return nil
	})
}

// deleteETJunction removes all et:{entryID}:* junction keys and their tge: reverses.
func deleteETJunction(txn *badgerdb.Txn, entryID string) error {
	var keys [][]byte
	prefix := pfxET(entryID)
	iterPrefix(txn, prefix, func(key []byte) bool {
		keys = append(keys, key)
		return true
	})
	for _, k := range keys {
		tagID := suffixAfter(k, prefix)
		_ = txn.Delete(kTGE(tagID, entryID))
		if err := txn.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

func (r *libraryEntryRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Remove external ID index.
		if old, err := getJSON[libEntryRecord](txn, kLE(id)); err == nil {
			for _, eid := range old.ExternalIDs {
				_ = txn.Delete(kEID("library_entry", eid.Source, eid.Value))
			}
		}

		// Remove tag junctions.
		_ = deleteETJunction(txn, id)

		// Remove entry-person junctions and their reverses.
		if err := deleteEPJunctions(txn, id); err != nil {
			return err
		}

		// Orphan any child entries (clear parent_id) — full scan acceptable for Phase 1.
		var children []libEntryRecord
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxLE()
		it := txn.NewIterator(opts)
		for it.Seek(pfxLE()); it.ValidForPrefix(pfxLE()); it.Next() {
			var rec libEntryRecord
			_ = it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) })
			if rec.ParentID == id {
				children = append(children, rec)
			}
		}
		it.Close()
		for _, child := range children {
			child.ParentID = ""
			_ = setJSON(txn, kLE(child.ID), child)
		}

		return txn.Delete(kLE(id))
	})
}

func deleteEPJunctions(txn *badgerdb.Txn, entryID string) error {
	var keys [][]byte
	prefix := pfxEP(entryID)
	iterPrefix(txn, prefix, func(key []byte) bool {
		keys = append(keys, key)
		return true
	})
	for _, k := range keys {
		parts := splitN(suffixAfter(k, prefix), 2)
		if len(parts) >= 2 {
			personID, role := parts[0], parts[1]
			_ = txn.Delete(kEPP(personID, entryID, role))
		}
		if err := txn.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

func (r *libraryEntryRepo) DeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
	impact := &domain.DeletionImpact{Mode: domain.DeletionModeDestroy, Impacts: []domain.DeletionImpactRow{}}

	err := r.db.View(func(txn *badgerdb.Txn) error {
		var groupCount, itemCount int

		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxGRP()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pfxGRP()); it.ValidForPrefix(pfxGRP()); it.Next() {
			var rec groupRecord
			_ = it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) })
			if rec.LibraryEntryID == id {
				groupCount++
			}
		}

		opts2 := badgerdb.DefaultIteratorOptions
		opts2.Prefix = pfxITM()
		it2 := txn.NewIterator(opts2)
		defer it2.Close()
		for it2.Seek(pfxITM()); it2.ValidForPrefix(pfxITM()); it2.Next() {
			var rec itemRecord
			_ = it2.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) })
			if rec.LibraryEntryID == id {
				itemCount++
			}
		}

		if groupCount > 0 {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind: "group", Count: groupCount, Label: "Groups",
			})
		}
		if itemCount > 0 {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind: "item", Count: itemCount, Label: "Items",
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("deletion impact for entry %s: %w", id, err)
	}
	return impact, nil
}

func (r *libraryEntryRepo) GetPeople(_ context.Context, entryID string) ([]domain.EntryPerson, error) {
	var people []domain.EntryPerson
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var err error
		people, err = loadEntryPeople(txn, entryID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get people for entry %s: %w", entryID, err)
	}
	return people, nil
}

func (r *libraryEntryRepo) SavePerson(_ context.Context, entryID string, ep domain.EntryPerson) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		role := ep.Role
		junc := entryPersonJunction{
			StartDate: dateToStr(ep.StartDate),
			EndDate:   dateToStr(ep.EndDate),
		}
		if err := setJSON(txn, kEP(entryID, ep.PersonID, role), junc); err != nil {
			return err
		}
		return txn.Set(kEPP(ep.PersonID, entryID, role), []byte{})
	})
}

func (r *libraryEntryRepo) RemovePerson(_ context.Context, entryID, personID, role string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		_ = txn.Delete(kEP(entryID, personID, role))
		_ = txn.Delete(kEPP(personID, entryID, role))
		return nil
	})
}
