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

type itemRepo struct {
	db *badgerdb.DB
}

// NewItemRepo returns an ItemRepository backed by BadgerDB.
func NewItemRepo(db *badgerdb.DB) ports.ItemRepository {
	return &itemRepo{db: db}
}

var _ ports.ItemRepository = (*itemRepo)(nil)

func (r *itemRepo) Get(_ context.Context, id string) (*domain.Item, error) {
	var item *domain.Item
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var err error
		item, err = loadItem(txn, id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get item %s: %w", id, err)
	}
	return item, nil
}

func loadItem(txn *badgerdb.Txn, id string) (*domain.Item, error) {
	rec, err := getJSON[itemRecord](txn, kITM(id))
	if err != nil {
		return nil, err
	}
	return itemFromRecord(txn, rec), nil
}

func itemFromRecord(txn *badgerdb.Txn, rec *itemRecord) *domain.Item {
	date := strToDate(rec.Date)
	sortKey := rec.SortKey
	if sortKey == "" {
		sortKey = domain.ItemSortKey(date, rec.Sequence, rec.Title)
	}
	item := &domain.Item{
		ID:             rec.ID,
		ContentType:    domain.ContentType(rec.ContentType),
		LibraryEntryID: rec.LibraryEntryID,
		GroupID:        rec.GroupID,
		Title:          rec.Title,
		Overview:       rec.Overview,
		Date:           date,
		Sequence:       rec.Sequence,
		SortKey:        sortKey,
		RuntimeSeconds: rec.RuntimeSeconds,
		Monitored:      rec.Monitored,
		Status:         domain.ItemStatus(rec.Status),
		CoverPath:      rec.CoverPath,
		Metadata:       rec.Metadata,
		LockedFields:   rec.LockedFields,
		ExternalIDs:    fromExtIDRecords(rec.ExternalIDs),
		AddedAt:        strToTime(rec.AddedAt),
		UpdatedAt:      strToTime(rec.UpdatedAt),
	}

	// Tags
	for _, t := range rec.Tags {
		item.Tags = append(item.Tags, domain.Tag{
			ID:    t.ID,
			Key:   domain.TagKey(t.Key),
			Value: t.Value,
			Scope: domain.TagScope(t.Scope),
		})
	}

	// People (with loaded person stubs)
	for _, p := range rec.People {
		ip := domain.ItemPerson{
			PersonID: p.PersonID,
			Role:     domain.PersonRole(p.Role),
		}
		if txn != nil {
			ip.Person = loadPersonStub(txn, p.PersonID)
		}
		item.People = append(item.People, ip)
	}

	return item
}

func (r *itemRepo) List(_ context.Context, f ports.ItemFilter) ([]*domain.Item, int, error) {
	var results []*domain.Item

	err := r.db.View(func(txn *badgerdb.Txn) error {
		// For PersonID filter, collect qualifying item IDs via ipp: reverse index.
		var personItemIDs map[string]struct{}
		if f.PersonID != "" {
			personItemIDs = make(map[string]struct{})
			iterPrefix(txn, pfxIPP(f.PersonID), func(key []byte) bool {
				parts := splitN(suffixAfter(key, pfxIPP(f.PersonID)), 2)
				if len(parts) >= 1 {
					personItemIDs[parts[0]] = struct{}{}
				}
				return true
			})
		}

		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxITM()
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(pfxITM()); it.ValidForPrefix(pfxITM()); it.Next() {
			var rec itemRecord
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			}); err != nil {
				continue
			}

			if !matchesItemFilter(rec, f, personItemIDs) {
				continue
			}

			results = append(results, itemFromRecord(txn, &rec))
		}
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}

	sortItems(results, f)

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

// strFilterMatches returns false when filter is set but does not equal rec.
func strFilterMatches(filterVal, recVal string) bool {
	return filterVal == "" || filterVal == recVal
}

func itemContentTypeMatch(rec itemRecord, cts []domain.ContentType) bool {
	for _, ct := range cts {
		if rec.ContentType == string(ct) {
			return true
		}
	}
	return false
}

func itemTagIDsMatch(rec itemRecord, tagIDs []string) bool {
	tagSet := make(map[string]struct{}, len(rec.Tags))
	for _, t := range rec.Tags {
		tagSet[t.ID] = struct{}{}
	}
	for _, wantID := range tagIDs {
		if _, ok := tagSet[wantID]; !ok {
			return false
		}
	}
	return true
}

func matchesItemFilter(rec itemRecord, f ports.ItemFilter, personItemIDs map[string]struct{}) bool {
	if !strFilterMatches(f.LibraryEntryID, rec.LibraryEntryID) {
		return false
	}
	if !strFilterMatches(f.GroupID, rec.GroupID) {
		return false
	}
	if len(f.ContentTypes) > 0 && !itemContentTypeMatch(rec, f.ContentTypes) {
		return false
	}
	if !strFilterMatches(string(f.Status), rec.Status) {
		return false
	}
	if f.Monitored != nil && rec.Monitored != *f.Monitored {
		return false
	}
	if f.Search != "" && !strings.Contains(strings.ToLower(rec.Title), strings.ToLower(f.Search)) {
		return false
	}
	if personItemIDs != nil {
		if _, ok := personItemIDs[rec.ID]; !ok {
			return false
		}
	}
	if !matchesItemTagFilter(rec, f) {
		return false
	}
	return true
}

func matchesItemTagFilter(rec itemRecord, f ports.ItemFilter) bool {
	if len(f.TagIDs) > 0 && !itemTagIDsMatch(rec, f.TagIDs) {
		return false
	}
	if f.TagKey != "" || f.TagValue != "" {
		if !itemTagKeyValueMatch(rec, string(f.TagKey), f.TagValue) {
			return false
		}
	}
	return true
}

func itemTagKeyValueMatch(rec itemRecord, key, value string) bool {
	for _, t := range rec.Tags {
		keyOK := key == "" || t.Key == key
		valOK := value == "" || t.Value == value
		if keyOK && valOK {
			return true
		}
	}
	return false
}

func sortItems(items []*domain.Item, f ports.ItemFilter) {
	asc := strings.ToUpper(f.SortDir) == "ASC"
	sort.Slice(items, func(i, j int) bool {
		switch f.Sort {
		case "title":
			if asc {
				return items[i].Title < items[j].Title
			}
			return items[i].Title > items[j].Title
		default: // date — SortKey encodes (date|sequence|title) ascending
			if asc {
				return items[i].SortKey < items[j].SortKey
			}
			return items[i].SortKey > items[j].SortKey
		}
	})
}

func (r *itemRepo) Save(_ context.Context, item *domain.Item) error {
	if item.ID == "" {
		item.ID = newID()
	}
	item.ApplyDefaults()
	now := nowStr()
	if item.AddedAt.IsZero() {
		item.AddedAt = strToTime(now)
	}
	item.UpdatedAt = strToTime(now)

	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Clean up old external ID, tag, and person junction keys.
		if err := deleteItemJunctions(txn, item.ID); err != nil {
			return err
		}

		// Build storage record.
		tags := make([]itemTagRecord, 0, len(item.Tags))
		for _, t := range item.Tags {
			tags = append(tags, itemTagRecord{ID: t.ID, Key: string(t.Key), Value: t.Value, Scope: string(t.Scope)})
		}
		people := make([]itemPersonRecord, 0, len(item.People))
		for _, p := range item.People {
			people = append(people, itemPersonRecord{PersonID: p.PersonID, Role: string(p.Role)})
		}

		rec := itemRecord{
			ID:             item.ID,
			ContentType:    string(item.ContentType),
			LibraryEntryID: item.LibraryEntryID,
			GroupID:        item.GroupID,
			Title:          item.Title,
			Overview:       item.Overview,
			Date:           dateToStr(item.Date),
			Sequence:       item.Sequence,
			SortKey:        item.SortKey,
			RuntimeSeconds: item.RuntimeSeconds,
			Monitored:      item.Monitored,
			Status:         string(item.Status),
			CoverPath:      item.CoverPath,
			Metadata:       item.Metadata,
			LockedFields:   item.LockedFields,
			ExternalIDs:    toExtIDRecords(item.ExternalIDs),
			Tags:           tags,
			People:         people,
			AddedAt:        timeToStr(item.AddedAt),
			UpdatedAt:      now,
		}
		if err := setJSON(txn, kITM(item.ID), rec); err != nil {
			return err
		}

		// Write external ID index entries.
		for _, eid := range item.ExternalIDs {
			if err := txn.Set(kEID("item", string(eid.Source), eid.Value), []byte(item.ID)); err != nil {
				return err
			}
		}

		// Write tag junction keys.
		for _, tag := range item.Tags {
			if err := txn.Set(kIT(item.ID, tag.ID), []byte{}); err != nil {
				return err
			}
			if err := txn.Set(kTGI(tag.ID, item.ID), []byte{}); err != nil {
				return err
			}
		}

		// Write person junction keys.
		for _, p := range item.People {
			role := string(p.Role)
			if err := txn.Set(kIP(item.ID, p.PersonID, role), []byte{}); err != nil {
				return err
			}
			if err := txn.Set(kIPP(p.PersonID, item.ID, role), []byte{}); err != nil {
				return err
			}
		}

		return nil
	})
}

func deleteItemJunctions(txn *badgerdb.Txn, itemID string) error {
	// Old external IDs
	if old, err := getJSON[itemRecord](txn, kITM(itemID)); err == nil {
		for _, eid := range old.ExternalIDs {
			_ = txn.Delete(kEID("item", eid.Source, eid.Value))
		}
	}

	// Old tag junction keys.
	itPrefix := pfxIT(itemID)
	var itKeys [][]byte
	iterPrefix(txn, itPrefix, func(key []byte) bool {
		itKeys = append(itKeys, key)
		return true
	})
	for _, k := range itKeys {
		tagID := suffixAfter(k, itPrefix)
		_ = txn.Delete(kTGI(tagID, itemID))
		_ = txn.Delete(k)
	}

	// Old person junction keys.
	ipPrefix := pfxIP(itemID)
	var ipKeys [][]byte
	iterPrefix(txn, ipPrefix, func(key []byte) bool {
		ipKeys = append(ipKeys, key)
		return true
	})
	for _, k := range ipKeys {
		parts := splitN(suffixAfter(k, ipPrefix), 2)
		if len(parts) >= 2 {
			personID, role := parts[0], parts[1]
			_ = txn.Delete(kIPP(personID, itemID, role))
		}
		_ = txn.Delete(k)
	}

	return nil
}

func (r *itemRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if err := deleteItemJunctions(txn, id); err != nil {
			return err
		}
		return txn.Delete(kITM(id))
	})
}

func (r *itemRepo) DeleteByGroup(ctx context.Context, groupID string) error {
	var itemIDs []string
	if err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxITM()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pfxITM()); it.ValidForPrefix(pfxITM()); it.Next() {
			var rec itemRecord
			if err := it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) }); err != nil {
				continue
			}
			if rec.GroupID == groupID {
				itemIDs = append(itemIDs, rec.ID)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("collect items for group %s: %w", groupID, err)
	}

	for _, id := range itemIDs {
		if err := r.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *itemRepo) DeleteByLibraryEntry(ctx context.Context, entryID string) error {
	var itemIDs []string
	if err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxITM()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pfxITM()); it.ValidForPrefix(pfxITM()); it.Next() {
			var rec itemRecord
			if err := it.Item().Value(func(val []byte) error { return json.Unmarshal(val, &rec) }); err != nil {
				continue
			}
			if rec.LibraryEntryID == entryID {
				itemIDs = append(itemIDs, rec.ID)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("collect items for entry %s: %w", entryID, err)
	}

	for _, id := range itemIDs {
		if err := r.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *itemRepo) DeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
	impact := &domain.DeletionImpact{Mode: domain.DeletionModeDestroy, Impacts: []domain.DeletionImpactRow{}}

	err := r.db.View(func(txn *badgerdb.Txn) error {
		rec, err := getJSON[itemRecord](txn, kITM(id))
		if err != nil {
			return err
		}
		// Check media file
		_, mfErr := getJSON[mediaFileRecord](txn, kMFI(rec.ID))
		_ = mfErr // media file may not exist; that's fine

		// Count via mfi: index
		if _, err := txn.Get(kMFI(id)); err == nil {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind: "media_file", Count: 1, Label: "Files",
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("deletion impact for item %s: %w", id, err)
	}
	return impact, nil
}
