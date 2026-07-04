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

type unmatchedFileRepo struct {
	db *badgerdb.DB
}

// NewUnmatchedFileRepo returns an UnmatchedFileRepository backed by BadgerDB.
func NewUnmatchedFileRepo(db *badgerdb.DB) ports.UnmatchedFileRepository {
	return &unmatchedFileRepo{db: db}
}

var _ ports.UnmatchedFileRepository = (*unmatchedFileRepo)(nil)

func (r *unmatchedFileRepo) List(_ context.Context, f ports.UnmatchedFilter) ([]*domain.UnmatchedFile, error) {
	var results []*domain.UnmatchedFile

	err := r.db.View(func(txn *badgerdb.Txn) error {
		iterPrefixValues(txn, pfxUMF(), func(_, val []byte) bool {
			var rec unmatchedFileRecord
			if json.Unmarshal(val, &rec) != nil {
				return true
			}
			if f.ContentType != "" && rec.ContentType != string(f.ContentType) {
				return true
			}
			if f.Status != "" && rec.Status != string(f.Status) {
				return true
			}
			if f.Path != "" && rec.Path != f.Path {
				return true
			}
			results = append(results, unmatchedFileFromRecord(&rec))
			return true
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list unmatched files: %w", err)
	}
	return results, nil
}

func (r *unmatchedFileRepo) Get(_ context.Context, id string) (*domain.UnmatchedFile, error) {
	var uf *domain.UnmatchedFile
	err := r.db.View(func(txn *badgerdb.Txn) error {
		rec, err := getJSON[unmatchedFileRecord](txn, kUMF(id))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		uf = unmatchedFileFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get unmatched file %s: %w", id, err)
	}
	return uf, nil
}

func (r *unmatchedFileRepo) Save(_ context.Context, f *domain.UnmatchedFile) error {
	if f.ID == "" {
		f.ID = newID()
	}
	if f.DiscoveredAt.IsZero() {
		f.DiscoveredAt = strToTime(nowStr())
	}
	if f.Status == "" {
		f.Status = domain.UnmatchedPending
	}

	rec := unmatchedFileToRecord(f)
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if err := setJSON(txn, kUMF(f.ID), rec); err != nil {
			return err
		}
		return nil
	})
}

func (r *unmatchedFileRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if err := txn.Delete(kUMF(id)); err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
			return fmt.Errorf("delete unmatched file %s: %w", id, err)
		}
		return nil
	})
}

func unmatchedFileFromRecord(rec *unmatchedFileRecord) *domain.UnmatchedFile {
	status := domain.UnmatchedStatus(rec.Status)
	if status == "" {
		status = domain.UnmatchedPending
	}
	uf := &domain.UnmatchedFile{
		ID:            rec.ID,
		Path:          rec.Path,
		Size:          rec.Size,
		ContentType:   domain.ContentType(rec.ContentType),
		Status:        status,
		DiscoveredAt:  strToTime(rec.DiscoveredAt),
		DuplicateOf:   rec.DuplicateOf,
		ThumbnailPath: rec.ThumbnailPath,
	}
	if rec.Fingerprint != nil {
		uf.Fingerprint = &domain.Fingerprint{
			OSHash:       rec.Fingerprint.OSHash,
			PHash:        rec.Fingerprint.PHash,
			AcoustID:     rec.Fingerprint.AcoustID,
			EmbeddedTags: rec.Fingerprint.EmbeddedTags,
			ISBN:         rec.Fingerprint.ISBN,
		}
	}
	for _, c := range rec.Candidates {
		mc := domain.MatchCandidate{
			ExternalItem: c.ExternalItem,
			Confidence:   c.Confidence,
			Source:       c.Source,
		}
		if c.ItemID != "" {
			mc.Item = &domain.Item{ID: c.ItemID}
		}
		uf.Candidates = append(uf.Candidates, mc)
	}
	return uf
}

func unmatchedFileToRecord(f *domain.UnmatchedFile) unmatchedFileRecord {
	rec := unmatchedFileRecord{
		ID:            f.ID,
		Path:          f.Path,
		Size:          f.Size,
		ContentType:   string(f.ContentType),
		Status:        string(f.Status),
		DiscoveredAt:  timeToStr(f.DiscoveredAt),
		DuplicateOf:   f.DuplicateOf,
		ThumbnailPath: f.ThumbnailPath,
	}
	if f.Fingerprint != nil {
		rec.Fingerprint = &fingerprintRecord{
			OSHash:       f.Fingerprint.OSHash,
			PHash:        f.Fingerprint.PHash,
			AcoustID:     f.Fingerprint.AcoustID,
			EmbeddedTags: f.Fingerprint.EmbeddedTags,
			ISBN:         f.Fingerprint.ISBN,
		}
	}
	for _, c := range f.Candidates {
		cr := matchCandidateRecord{
			ExternalItem: c.ExternalItem,
			Confidence:   c.Confidence,
			Source:       c.Source,
		}
		if c.Item != nil {
			cr.ItemID = c.Item.ID
		}
		rec.Candidates = append(rec.Candidates, cr)
	}
	return rec
}
