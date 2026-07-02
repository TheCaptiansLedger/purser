package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"

	badgerdb "github.com/dgraph-io/badger/v4"
)

type groupRepo struct {
	db *badgerdb.DB
}

// NewGroupRepo returns a GroupRepository backed by BadgerDB.
func NewGroupRepo(db *badgerdb.DB) ports.GroupRepository {
	return &groupRepo{db: db}
}

var _ ports.GroupRepository = (*groupRepo)(nil)

func (r *groupRepo) Get(_ context.Context, id string) (*domain.Group, error) {
	var g *domain.Group
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var err error
		g, err = loadGroup(txn, id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get group %s: %w", id, err)
	}
	return g, nil
}

func loadGroup(txn *badgerdb.Txn, id string) (*domain.Group, error) {
	rec, err := getJSON[groupRecord](txn, kGRP(id))
	if err != nil {
		return nil, err
	}
	g := groupFromRecord(rec)

	tags, err := loadTagsForPrefix(txn, pfxGT(id))
	if err != nil {
		return nil, err
	}
	g.Tags = tags

	return g, nil
}

func groupFromRecord(rec *groupRecord) *domain.Group {
	sortKey := rec.SortKey
	if sortKey == "" {
		sortKey = domain.GroupSortKey(rec.Number, rec.Title)
	}
	return &domain.Group{
		ID:             rec.ID,
		LibraryEntryID: rec.LibraryEntryID,
		Title:          rec.Title,
		SortName:       rec.SortName,
		SortKey:        sortKey,
		Number:         rec.Number,
		Year:           rec.Year,
		Overview:       rec.Overview,
		Monitored:      rec.Monitored,
		MonitorMode:    domain.MonitorMode(rec.MonitorMode),
		Metadata:       rec.Metadata,
		LockedFields:   rec.LockedFields,
		CoverPath:      rec.CoverPath,
		ExternalIDs:    fromExtIDRecords(rec.ExternalIDs),
	}
}

func (r *groupRepo) List(_ context.Context, f ports.GroupFilter) ([]*domain.Group, error) {
	var results []*domain.Group

	err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxGRP()
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(pfxGRP()); it.ValidForPrefix(pfxGRP()); it.Next() {
			var rec groupRecord
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			}); err != nil {
				continue
			}

			if !matchesGroupFilter(rec, f) {
				continue
			}

			results = append(results, groupFromRecord(&rec))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SortKey < results[j].SortKey
	})

	return results, nil
}

func matchesGroupFilter(rec groupRecord, f ports.GroupFilter) bool {
	if f.LibraryEntryID != "" && rec.LibraryEntryID != f.LibraryEntryID {
		return false
	}
	if f.Monitored != nil && rec.Monitored != *f.Monitored {
		return false
	}
	return true
}

func (r *groupRepo) Save(_ context.Context, g *domain.Group) error {
	if g.ID == "" {
		g.ID = newID()
	}
	g.ApplyDefaults()

	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Denormalize content_type from parent entry.
		contentType := ""
		if entry, err := getJSON[libEntryRecord](txn, kLE(g.LibraryEntryID)); err == nil {
			contentType = entry.ContentType
		}

		// Clean up old external ID index entries.
		if old, err := getJSON[groupRecord](txn, kGRP(g.ID)); err == nil {
			for _, eid := range old.ExternalIDs {
				_ = txn.Delete(kEID("group", eid.Source, eid.Value))
			}
		}

		rec := groupRecord{
			ID:             g.ID,
			LibraryEntryID: g.LibraryEntryID,
			ContentType:    contentType,
			Title:          g.Title,
			SortName:       g.SortName,
			SortKey:        g.SortKey,
			Number:         g.Number,
			Year:           g.Year,
			Overview:       g.Overview,
			Monitored:      g.Monitored,
			MonitorMode:    string(g.MonitorMode),
			Metadata:       g.Metadata,
			LockedFields:   g.LockedFields,
			CoverPath:      g.CoverPath,
			ExternalIDs:    toExtIDRecords(g.ExternalIDs),
		}
		if err := setJSON(txn, kGRP(g.ID), rec); err != nil {
			return err
		}

		// Write new external ID index entries.
		for _, eid := range g.ExternalIDs {
			if err := txn.Set(kEID("group", string(eid.Source), eid.Value), []byte(g.ID)); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *groupRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Clean up external IDs.
		if old, err := getJSON[groupRecord](txn, kGRP(id)); err == nil {
			for _, eid := range old.ExternalIDs {
				_ = txn.Delete(kEID("group", eid.Source, eid.Value))
			}
		}

		// Remove group tag junctions and reverses.
		if err := deleteGTJunction(txn, id); err != nil {
			return err
		}

		return txn.Delete(kGRP(id))
	})
}

// deleteGTJunction removes all gt:{groupID}:* and their tgg: reverses.
func deleteGTJunction(txn *badgerdb.Txn, groupID string) error {
	var keys [][]byte
	prefix := pfxGT(groupID)
	iterPrefix(txn, prefix, func(key []byte) bool {
		keys = append(keys, key)
		return true
	})
	for _, k := range keys {
		tagID := suffixAfter(k, prefix)
		_ = txn.Delete(kTGG(tagID, groupID))
		if err := txn.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

func (r *groupRepo) DeleteByLibraryEntry(ctx context.Context, entryID string) error {
	// Collect matching group IDs in a read transaction first.
	var groupIDs []string
	if err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxGRP()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pfxGRP()); it.ValidForPrefix(pfxGRP()); it.Next() {
			var rec groupRecord
			if err := it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) }); err != nil {
				continue
			}
			if rec.LibraryEntryID == entryID {
				groupIDs = append(groupIDs, rec.ID)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("collect groups for entry %s: %w", entryID, err)
	}

	for _, id := range groupIDs {
		if err := r.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *groupRepo) DeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
	impact := &domain.DeletionImpact{Mode: domain.DeletionModeDestroy, Impacts: []domain.DeletionImpactRow{}}

	err := r.db.View(func(txn *badgerdb.Txn) error {
		var itemCount int
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxITM()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pfxITM()); it.ValidForPrefix(pfxITM()); it.Next() {
			var rec itemRecord
			_ = it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) })
			if rec.GroupID == id {
				itemCount++
			}
		}
		if itemCount > 0 {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind: "item", Count: itemCount, Label: "Items",
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("deletion impact for group %s: %w", id, err)
	}
	return impact, nil
}
