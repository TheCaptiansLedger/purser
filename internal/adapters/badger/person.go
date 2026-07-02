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

type personRepo struct {
	db *badgerdb.DB
}

// NewPersonRepo returns a PersonRepository backed by BadgerDB.
func NewPersonRepo(db *badgerdb.DB) ports.PersonRepository {
	return &personRepo{db: db}
}

var _ ports.PersonRepository = (*personRepo)(nil)

func (r *personRepo) Get(_ context.Context, id string) (*domain.Person, error) {
	var p *domain.Person
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var err error
		p, err = loadPerson(txn, id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("get person %s: %w", id, err)
	}
	return p, nil
}

func loadPerson(txn *badgerdb.Txn, id string) (*domain.Person, error) {
	rec, err := getJSON[personRecord](txn, kPER(id))
	if err != nil {
		return nil, err
	}
	return personFromRecord(rec), nil
}

func personFromRecord(rec *personRecord) *domain.Person {
	p := &domain.Person{
		ID:           rec.ID,
		Name:         rec.Name,
		SortName:     rec.SortName,
		Overview:     rec.Overview,
		Monitored:    rec.Monitored,
		MonitorMode:  domain.MonitorMode(rec.MonitorMode),
		ImagePath:    rec.ImagePath,
		Metadata:     rec.Metadata,
		LockedFields: rec.LockedFields,
		Aliases:      rec.Aliases,
		AddedAt:      strToTime(rec.AddedAt),
		ExternalIDs:  fromExtIDRecords(rec.ExternalIDs),
	}
	for _, r := range rec.Roles {
		p.Roles = append(p.Roles, domain.PersonRole(r))
	}
	return p
}

// collectPersonIDsByRole returns person IDs indexed under the given role.
func (r *personRepo) collectPersonIDsByRole(role domain.PersonRole) (map[string]struct{}, error) {
	roleStr := string(role)
	ids := make(map[string]struct{})
	err := r.db.View(func(txn *badgerdb.Txn) error {
		iterPrefix(txn, pfxPRI(roleStr), func(key []byte) bool {
			ids[suffixAfter(key, pfxPRI(roleStr))] = struct{}{}
			return true
		})
		return nil
	})
	return ids, err
}

// collectPersonIDsByContentType returns person IDs linked to items/entries of
// any of the given content types.
func (r *personRepo) collectPersonIDsByContentType(cts []domain.ContentType) (map[string]struct{}, error) {
	ctSet := make(map[string]struct{}, len(cts))
	for _, ct := range cts {
		ctSet[string(ct)] = struct{}{}
	}
	ids := make(map[string]struct{})
	err := r.db.View(func(txn *badgerdb.Txn) error {
		collectPersonIDsFromItems(txn, ctSet, ids)
		collectPersonIDsFromEntries(txn, ctSet, ids)
		return nil
	})
	return ids, err
}

func collectPersonIDsFromItems(txn *badgerdb.Txn, ctSet, ids map[string]struct{}) {
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
		for _, p := range rec.People {
			ids[p.PersonID] = struct{}{}
		}
	}
}

func collectPersonIDsFromEntries(txn *badgerdb.Txn, ctSet, ids map[string]struct{}) {
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
		iterPrefix(txn, pfxEP(rec.ID), func(key []byte) bool {
			parts := splitN(suffixAfter(key, pfxEP(rec.ID)), 2)
			if len(parts) >= 1 {
				ids[parts[0]] = struct{}{}
			}
			return true
		})
	}
}

func (r *personRepo) List(_ context.Context, f ports.PersonFilter) ([]*domain.Person, int, error) {
	var rolePersonIDs map[string]struct{}
	if f.Role != "" {
		ids, err := r.collectPersonIDsByRole(f.Role)
		if err != nil {
			return nil, 0, fmt.Errorf("list people by role: %w", err)
		}
		rolePersonIDs = ids
	}

	var ctPersonIDs map[string]struct{}
	if len(f.ContentTypes) > 0 {
		ids, err := r.collectPersonIDsByContentType(f.ContentTypes)
		if err != nil {
			return nil, 0, fmt.Errorf("list people by content type: %w", err)
		}
		ctPersonIDs = ids
	}

	var results []*domain.Person
	err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxPER()
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(pfxPER()); it.ValidForPrefix(pfxPER()); it.Next() {
			var rec personRecord
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			}); err != nil {
				continue
			}

			if !matchesPersonFilter(txn, rec, f, rolePersonIDs, ctPersonIDs) {
				continue
			}

			results = append(results, personFromRecord(&rec))
		}
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list people: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		si := results[i].SortName
		if si == "" {
			si = results[i].Name
		}
		sj := results[j].SortName
		if sj == "" {
			sj = results[j].Name
		}
		return si < sj
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

func matchesPersonFilter(txn *badgerdb.Txn, rec personRecord, f ports.PersonFilter, roleIDs, ctIDs map[string]struct{}) bool {
	if f.Monitored != nil && rec.Monitored != *f.Monitored {
		return false
	}
	if roleIDs != nil {
		if _, ok := roleIDs[rec.ID]; !ok {
			return false
		}
	}
	if ctIDs != nil {
		if _, ok := ctIDs[rec.ID]; !ok {
			return false
		}
	}
	if f.Unlinked && personHasLinks(txn, rec.ID) {
		return false
	}
	if f.Search != "" {
		return personMatchesSearch(rec, f.Search)
	}
	return true
}

func personHasLinks(txn *badgerdb.Txn, personID string) bool {
	found := false
	iterPrefix(txn, pfxIPP(personID), func(_ []byte) bool {
		found = true
		return false
	})
	if found {
		return true
	}
	iterPrefix(txn, pfxEPP(personID), func(_ []byte) bool {
		found = true
		return false
	})
	return found
}

func personMatchesSearch(rec personRecord, search string) bool {
	lower := strings.ToLower(search)
	if strings.Contains(strings.ToLower(rec.Name), lower) {
		return true
	}
	for _, alias := range rec.Aliases {
		if strings.Contains(strings.ToLower(alias), lower) {
			return true
		}
	}
	return false
}

func (r *personRepo) ListRoles(_ context.Context) ([]domain.PersonRoleCount, error) {
	roleCounts := make(map[string]map[string]struct{}) // role → set of personIDs

	err := r.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.Prefix = pfxPER()
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(pfxPER()); it.ValidForPrefix(pfxPER()); it.Next() {
			var rec personRecord
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &rec)
			}); err != nil {
				continue
			}
			for _, role := range rec.Roles {
				if roleCounts[role] == nil {
					roleCounts[role] = make(map[string]struct{})
				}
				roleCounts[role][rec.ID] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list person roles: %w", err)
	}

	out := make([]domain.PersonRoleCount, 0, len(roleCounts))
	for role, personSet := range roleCounts {
		out = append(out, domain.PersonRoleCount{
			Role:  domain.PersonRole(role),
			Count: len(personSet),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out, nil
}

func (r *personRepo) Save(_ context.Context, p *domain.Person) error {
	if p.ID == "" {
		p.ID = newID()
	}
	if p.AddedAt.IsZero() {
		p.AddedAt = strToTime(nowStr())
	}

	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Clean up old external ID index and role index entries.
		if old, err := getJSON[personRecord](txn, kPER(p.ID)); err == nil {
			for _, eid := range old.ExternalIDs {
				_ = txn.Delete(kEID("person", eid.Source, eid.Value))
			}
			for _, role := range old.Roles {
				_ = txn.Delete(kPRI(role, p.ID))
			}
		}

		roles := make([]string, 0, len(p.Roles))
		for _, r := range p.Roles {
			roles = append(roles, string(r))
		}
		sort.Strings(roles)

		rec := personRecord{
			ID:           p.ID,
			Name:         p.Name,
			SortName:     p.SortName,
			Overview:     p.Overview,
			Monitored:    p.Monitored,
			MonitorMode:  string(p.MonitorMode),
			ImagePath:    p.ImagePath,
			Metadata:     p.Metadata,
			LockedFields: p.LockedFields,
			Aliases:      p.Aliases,
			Roles:        roles,
			ExternalIDs:  toExtIDRecords(p.ExternalIDs),
			AddedAt:      timeToStr(p.AddedAt),
		}
		if err := setJSON(txn, kPER(p.ID), rec); err != nil {
			return err
		}

		for _, eid := range p.ExternalIDs {
			if err := txn.Set(kEID("person", string(eid.Source), eid.Value), []byte(p.ID)); err != nil {
				return err
			}
		}

		for _, role := range roles {
			if err := txn.Set(kPRI(role, p.ID), []byte{}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *personRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if old, err := getJSON[personRecord](txn, kPER(id)); err == nil {
			for _, eid := range old.ExternalIDs {
				_ = txn.Delete(kEID("person", eid.Source, eid.Value))
			}
			for _, role := range old.Roles {
				_ = txn.Delete(kPRI(role, id))
			}
		}

		_ = deletePrefix(txn, pfxEPP(id))
		_ = deletePrefix(txn, pfxIPP(id))

		return txn.Delete(kPER(id))
	})
}

func (r *personRepo) DeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
	impact := &domain.DeletionImpact{Mode: domain.DeletionModeUnlink, Impacts: []domain.DeletionImpactRow{}}

	err := r.db.View(func(txn *badgerdb.Txn) error {
		itemCTCounts := make(map[string]int)
		iterPrefix(txn, pfxIPP(id), func(key []byte) bool {
			parts := splitN(suffixAfter(key, pfxIPP(id)), 2)
			if len(parts) < 1 {
				return true
			}
			rec, err := getJSON[itemRecord](txn, kITM(parts[0]))
			if err != nil {
				return true
			}
			itemCTCounts[rec.ContentType]++
			return true
		})
		for ct, count := range itemCTCounts {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind:  "item_" + ct,
				Count: count,
				Label: domain.ContentType(ct).ItemLabel(),
			})
		}

		entryKindCounts := make(map[string]int)
		iterPrefix(txn, pfxEPP(id), func(key []byte) bool {
			parts := splitN(suffixAfter(key, pfxEPP(id)), 2)
			if len(parts) < 1 {
				return true
			}
			rec, err := getJSON[libEntryRecord](txn, kLE(parts[0]))
			if err != nil {
				return true
			}
			entryKindCounts[rec.Kind]++
			return true
		})
		for kind, count := range entryKindCounts {
			impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
				Kind:  "entry_" + kind,
				Count: count,
				Label: domain.Kind(kind).EntryLabel(),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("deletion impact for person %s: %w", id, err)
	}
	return impact, nil
}
