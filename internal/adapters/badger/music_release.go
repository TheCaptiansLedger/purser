package badger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"

	badgerdb "github.com/dgraph-io/badger/v4"
)

type musicReleaseRepo struct{ db *badgerdb.DB }

// NewMusicReleaseRepo returns a MusicReleaseRepository backed by BadgerDB.
func NewMusicReleaseRepo(db *badgerdb.DB) ports.MusicReleaseRepository {
	return &musicReleaseRepo{db: db}
}

var _ ports.MusicReleaseRepository = (*musicReleaseRepo)(nil)

func (r *musicReleaseRepo) Get(_ context.Context, id string) (*domain.MusicRelease, error) {
	var rel *domain.MusicRelease
	err := r.db.View(func(txn *badgerdb.Txn) error {
		rec, err := getJSON[musicReleaseRecord](txn, kMREL(id))
		if err != nil {
			return err
		}
		rel = musicReleaseFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get music release %s: %w", id, err)
	}
	return rel, nil
}

func (r *musicReleaseRepo) GetByMBID(_ context.Context, mbid string) (*domain.MusicRelease, error) {
	var rel *domain.MusicRelease
	err := r.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kMRELMBID(mbid))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		var id string
		if err := item.Value(func(v []byte) error { id = string(v); return nil }); err != nil {
			return err
		}
		rec, err := getJSON[musicReleaseRecord](txn, kMREL(id))
		if err != nil {
			return err
		}
		rel = musicReleaseFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get music release by mbid %s: %w", mbid, err)
	}
	return rel, nil
}

func (r *musicReleaseRepo) GetByBarcode(_ context.Context, barcode string) (*domain.MusicRelease, error) {
	var rel *domain.MusicRelease
	err := r.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kMRELBarcode(barcode))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		var id string
		if err := item.Value(func(v []byte) error { id = string(v); return nil }); err != nil {
			return err
		}
		rec, err := getJSON[musicReleaseRecord](txn, kMREL(id))
		if err != nil {
			return err
		}
		rel = musicReleaseFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get music release by barcode %s: %w", barcode, err)
	}
	return rel, nil
}

func (r *musicReleaseRepo) ListByGroup(_ context.Context, groupID string) ([]*domain.MusicRelease, error) {
	return r.listByIndex(pfxMRELGrp(groupID), fmt.Sprintf("list music releases by group %s", groupID))
}

func (r *musicReleaseRepo) ListByEntry(_ context.Context, entryID string) ([]*domain.MusicRelease, error) {
	return r.listByIndex(pfxMRELEntry(entryID), fmt.Sprintf("list music releases by entry %s", entryID))
}

func (r *musicReleaseRepo) listByIndex(prefix []byte, errCtx string) ([]*domain.MusicRelease, error) {
	var results []*domain.MusicRelease
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var ids []string
		iterPrefix(txn, prefix, func(key []byte) bool {
			ids = append(ids, suffixAfter(key, prefix))
			return true
		})
		for _, id := range ids {
			rec, err := getJSON[musicReleaseRecord](txn, kMREL(id))
			if err != nil {
				continue
			}
			results = append(results, musicReleaseFromRecord(rec))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCtx, err)
	}
	return results, nil
}

// ListTracksByRelease scans all items and filters by Metadata["release_id"].
// Documented trade-off: a full item scan until a dedicated index is added.
func (r *musicReleaseRepo) ListTracksByRelease(_ context.Context, releaseID string) ([]*domain.Item, error) {
	var results []*domain.Item
	err := r.db.View(func(txn *badgerdb.Txn) error {
		iterPrefixValues(txn, pfxITM(), func(_, val []byte) bool {
			var rec itemRecord
			if json.Unmarshal(val, &rec) != nil {
				return true
			}
			if rid, ok := rec.Metadata["release_id"]; ok {
				if s, ok := rid.(string); ok && s == releaseID {
					results = append(results, itemFromRecord(txn, &rec))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list tracks by release %s: %w", releaseID, err)
	}
	return results, nil
}

func (r *musicReleaseRepo) Save(_ context.Context, rel *domain.MusicRelease) error {
	if rel.ID == "" {
		rel.ID = newID()
	}
	rel.ApplyDefaults()

	return r.db.Update(func(txn *badgerdb.Txn) error {
		if old, err := getJSON[musicReleaseRecord](txn, kMREL(rel.ID)); err == nil {
			_ = txn.Delete(kMRELGrp(old.GroupID, rel.ID))
			_ = txn.Delete(kMRELEntry(old.LibraryEntryID, rel.ID))
			for _, eid := range old.ExternalIDs {
				if eid.Source == string(domain.SourceMusicBrainz) {
					_ = txn.Delete(kMRELMBID(eid.Value))
				}
			}
			if old.Barcode != "" {
				_ = txn.Delete(kMRELBarcode(old.Barcode))
			}
		}

		rec := musicReleaseToRecord(rel)
		if err := setJSON(txn, kMREL(rel.ID), rec); err != nil {
			return err
		}
		if err := txn.Set(kMRELGrp(rel.GroupID, rel.ID), []byte(rel.ID)); err != nil {
			return err
		}
		if rel.LibraryEntryID != "" {
			if err := txn.Set(kMRELEntry(rel.LibraryEntryID, rel.ID), []byte(rel.ID)); err != nil {
				return err
			}
		}
		for _, eid := range rel.ExternalIDs {
			if eid.Source == domain.SourceMusicBrainz {
				if err := txn.Set(kMRELMBID(eid.Value), []byte(rel.ID)); err != nil {
					return err
				}
			}
		}
		if rel.Barcode != "" {
			if err := txn.Set(kMRELBarcode(rel.Barcode), []byte(rel.ID)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *musicReleaseRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if old, err := getJSON[musicReleaseRecord](txn, kMREL(id)); err == nil {
			_ = txn.Delete(kMRELGrp(old.GroupID, id))
			_ = txn.Delete(kMRELEntry(old.LibraryEntryID, id))
			for _, eid := range old.ExternalIDs {
				if eid.Source == string(domain.SourceMusicBrainz) {
					_ = txn.Delete(kMRELMBID(eid.Value))
				}
			}
			if old.Barcode != "" {
				_ = txn.Delete(kMRELBarcode(old.Barcode))
			}
		}
		if err := txn.Delete(kMREL(id)); err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
			return fmt.Errorf("delete music release %s: %w", id, err)
		}
		return nil
	})
}

func musicReleaseFromRecord(rec *musicReleaseRecord) *domain.MusicRelease {
	return &domain.MusicRelease{
		ID:             rec.ID,
		GroupID:        rec.GroupID,
		LibraryEntryID: rec.LibraryEntryID,
		Title:          rec.Title,
		Country:        rec.Country,
		Date:           strToDate(rec.Date),
		Label:          rec.Label,
		CatalogNumber:  rec.CatalogNumber,
		Barcode:        rec.Barcode,
		Format:         rec.Format,
		MediumCount:    rec.MediumCount,
		TrackCount:     rec.TrackCount,
		IsDefault:      rec.IsDefault,
		Monitored:      rec.Monitored,
		Status:         domain.ReleaseStatus(rec.Status),
		ExternalIDs:    fromExtIDRecords(rec.ExternalIDs),
		CoverPath:      rec.CoverPath,
		AddedAt:        strToTime(rec.AddedAt),
		UpdatedAt:      strToTime(rec.UpdatedAt),
	}
}

func musicReleaseToRecord(rel *domain.MusicRelease) musicReleaseRecord {
	return musicReleaseRecord{
		ID:             rel.ID,
		GroupID:        rel.GroupID,
		LibraryEntryID: rel.LibraryEntryID,
		Title:          rel.Title,
		Country:        rel.Country,
		Date:           dateToStr(rel.Date),
		Label:          rel.Label,
		CatalogNumber:  rel.CatalogNumber,
		Barcode:        rel.Barcode,
		Format:         rel.Format,
		MediumCount:    rel.MediumCount,
		TrackCount:     rel.TrackCount,
		IsDefault:      rel.IsDefault,
		Monitored:      rel.Monitored,
		Status:         string(rel.Status),
		ExternalIDs:    toExtIDRecords(rel.ExternalIDs),
		CoverPath:      rel.CoverPath,
		AddedAt:        timeToStr(rel.AddedAt),
		UpdatedAt:      timeToStr(rel.UpdatedAt),
	}
}
