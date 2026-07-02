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

type tagRepo struct {
	db *badgerdb.DB
}

// NewTagRepo returns a TagRepository backed by BadgerDB.
func NewTagRepo(db *badgerdb.DB) ports.TagRepository {
	return &tagRepo{db: db}
}

var _ ports.TagRepository = (*tagRepo)(nil)

func (r *tagRepo) Get(_ context.Context, id string) (*domain.Tag, error) {
	var t *domain.Tag
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var err error
		t, err = loadTagByID(txn, id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get tag %s: %w", id, err)
	}
	return t, nil
}

// collectTagIDsByContentType returns tag IDs used by entries, groups, or items
// whose content_type is in cts.
func (r *tagRepo) collectTagIDsByContentType(cts []domain.ContentType) (map[string]struct{}, error) {
	ctSet := make(map[string]struct{}, len(cts))
	for _, ct := range cts {
		ctSet[string(ct)] = struct{}{}
	}
	ids := make(map[string]struct{})
	err := r.db.View(func(txn *badgerdb.Txn) error {
		collectTagIDsFromEntries(txn, ctSet, ids)
		collectTagIDsFromGroups(txn, ctSet, ids)
		collectTagIDsFromItems(txn, ctSet, ids)
		return nil
	})
	return ids, err
}

func collectTagIDsFromEntries(txn *badgerdb.Txn, ctSet, ids map[string]struct{}) {
	opts := badgerdb.DefaultIteratorOptions
	opts.Prefix = pfxLE()
	it := txn.NewIterator(opts)
	defer it.Close()
	for it.Seek(pfxLE()); it.ValidForPrefix(pfxLE()); it.Next() {
		var rec libEntryRecord
		if err := it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) }); err != nil {
			continue
		}
		if _, ok := ctSet[rec.ContentType]; !ok {
			continue
		}
		iterPrefix(txn, pfxET(rec.ID), func(key []byte) bool {
			ids[suffixAfter(key, pfxET(rec.ID))] = struct{}{}
			return true
		})
	}
}

func collectTagIDsFromGroups(txn *badgerdb.Txn, ctSet, ids map[string]struct{}) {
	opts := badgerdb.DefaultIteratorOptions
	opts.Prefix = pfxGRP()
	it := txn.NewIterator(opts)
	defer it.Close()
	for it.Seek(pfxGRP()); it.ValidForPrefix(pfxGRP()); it.Next() {
		var rec groupRecord
		if err := it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) }); err != nil {
			continue
		}
		if _, ok := ctSet[rec.ContentType]; !ok {
			continue
		}
		iterPrefix(txn, pfxGT(rec.ID), func(key []byte) bool {
			ids[suffixAfter(key, pfxGT(rec.ID))] = struct{}{}
			return true
		})
	}
}

func collectTagIDsFromItems(txn *badgerdb.Txn, ctSet, ids map[string]struct{}) {
	opts := badgerdb.DefaultIteratorOptions
	opts.Prefix = pfxITM()
	it := txn.NewIterator(opts)
	defer it.Close()
	for it.Seek(pfxITM()); it.ValidForPrefix(pfxITM()); it.Next() {
		var rec itemRecord
		if err := it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) }); err != nil {
			continue
		}
		if _, ok := ctSet[rec.ContentType]; !ok {
			continue
		}
		iterPrefix(txn, pfxIT(rec.ID), func(key []byte) bool {
			ids[suffixAfter(key, pfxIT(rec.ID))] = struct{}{}
			return true
		})
	}
}

func (r *tagRepo) List(_ context.Context, f ports.TagFilter) ([]*domain.Tag, error) {
	var ctTagIDs map[string]struct{}
	if len(f.ContentTypes) > 0 {
		ids, err := r.collectTagIDsByContentType(f.ContentTypes)
		if err != nil {
			return nil, fmt.Errorf("list tags by content type: %w", err)
		}
		ctTagIDs = ids
	}

	var groupTagIDs map[string]struct{}
	if f.GroupID != "" {
		groupTagIDs = make(map[string]struct{})
		if err := r.db.View(func(txn *badgerdb.Txn) error {
			iterPrefix(txn, pfxGT(f.GroupID), func(key []byte) bool {
				groupTagIDs[suffixAfter(key, pfxGT(f.GroupID))] = struct{}{}
				return true
			})
			return nil
		}); err != nil {
			return nil, fmt.Errorf("list tags by group: %w", err)
		}
	}

	var results []*domain.Tag
	err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxTAG()
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(pfxTAG()); it.ValidForPrefix(pfxTAG()); it.Next() {
			var rec tagRecord
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			}); err != nil {
				continue
			}

			if !matchesTagFilter(rec, f, ctTagIDs, groupTagIDs) {
				continue
			}

			results = append(results, tagFromRecord(&rec))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SortKey < results[j].SortKey
	})
	return results, nil
}

func matchesTagFilter(rec tagRecord, f ports.TagFilter, ctTagIDs, groupTagIDs map[string]struct{}) bool {
	if f.Key != "" && rec.Key != string(f.Key) {
		return false
	}
	if f.Scope != "" && rec.Scope != string(f.Scope) {
		return false
	}
	if f.Value != "" && rec.Value != f.Value {
		return false
	}
	if ctTagIDs != nil {
		if _, ok := ctTagIDs[rec.ID]; !ok {
			return false
		}
	}
	if groupTagIDs != nil {
		if _, ok := groupTagIDs[rec.ID]; !ok {
			return false
		}
	}
	return true
}

func (r *tagRepo) Save(_ context.Context, t *domain.Tag) error {
	if t.ID == "" {
		t.ID = newID()
	}
	t.ApplyDefaults()
	rec := tagRecord{
		ID:      t.ID,
		Key:     string(t.Key),
		Value:   t.Value,
		Scope:   string(t.Scope),
		SortKey: t.SortKey,
	}
	return r.db.Update(func(txn *badgerdb.Txn) error {
		return setJSON(txn, kTAG(t.ID), rec)
	})
}

func (r *tagRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if err := deletePrefix(txn, pfxTGE(id)); err != nil {
			return err
		}
		if err := deletePrefix(txn, pfxTGG(id)); err != nil {
			return err
		}
		if err := deletePrefix(txn, pfxTGI(id)); err != nil {
			return err
		}
		return txn.Delete(kTAG(id))
	})
}

func (r *tagRepo) DeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
	impact := &domain.DeletionImpact{Mode: domain.DeletionModeUnlink, Impacts: []domain.DeletionImpactRow{}}

	err := r.db.View(func(txn *badgerdb.Txn) error {
		var entryCount, groupCount, itemCount int
		iterPrefix(txn, pfxTGE(id), func(_ []byte) bool { entryCount++; return true })
		iterPrefix(txn, pfxTGG(id), func(_ []byte) bool { groupCount++; return true })
		iterPrefix(txn, pfxTGI(id), func(_ []byte) bool { itemCount++; return true })

		if entryCount > 0 {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind: "entry", Count: entryCount, Label: "Entries",
			})
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
		return nil, fmt.Errorf("deletion impact for tag %s: %w", id, err)
	}
	return impact, nil
}

func (r *tagRepo) AddGroupTag(_ context.Context, groupID, tagID string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if err := txn.Set(kGT(groupID, tagID), []byte{}); err != nil {
			return err
		}
		return txn.Set(kTGG(tagID, groupID), []byte{})
	})
}

func (r *tagRepo) RemoveGroupTag(_ context.Context, groupID, tagID string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		_ = txn.Delete(kGT(groupID, tagID))
		_ = txn.Delete(kTGG(tagID, groupID))
		return nil
	})
}
