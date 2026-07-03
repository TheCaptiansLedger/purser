package badger

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"

	badgerdb "github.com/dgraph-io/badger/v4"
)

type mediaFileRepo struct {
	db *badgerdb.DB
}

// NewMediaFileRepo returns a MediaFileRepository backed by BadgerDB.
func NewMediaFileRepo(db *badgerdb.DB) ports.MediaFileRepository {
	return &mediaFileRepo{db: db}
}

var _ ports.MediaFileRepository = (*mediaFileRepo)(nil)

func (r *mediaFileRepo) GetByItemID(_ context.Context, itemID string) (*domain.MediaFile, error) {
	var mf *domain.MediaFile
	err := r.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kMFI(itemID))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		var mfID string
		if err := item.Value(func(val []byte) error {
			mfID = string(val)
			return nil
		}); err != nil {
			return err
		}
		rec, err := getJSON[mediaFileRecord](txn, kMF(mfID))
		if err != nil {
			return err
		}
		mf = mediaFileFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get media file by item %s: %w", itemID, err)
	}
	return mf, nil
}

func (r *mediaFileRepo) GetByOSHash(_ context.Context, hash string) (*domain.MediaFile, error) {
	var mf *domain.MediaFile
	err := r.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kMFH(hash))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		var mfID string
		if err := item.Value(func(val []byte) error {
			mfID = string(val)
			return nil
		}); err != nil {
			return err
		}
		rec, err := getJSON[mediaFileRecord](txn, kMF(mfID))
		if err != nil {
			return err
		}
		mf = mediaFileFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get media file by hash %s: %w", hash, err)
	}
	return mf, nil
}

func mediaFileFromRecord(rec *mediaFileRecord) *domain.MediaFile {
	mc := domain.MatchConfidence(rec.MatchConfidence)
	if mc == "" {
		mc = domain.MatchNameMatched
	}
	return &domain.MediaFile{
		ID:              rec.ID,
		ItemID:          rec.ItemID,
		Path:            rec.Path,
		Size:            rec.Size,
		OSHash:          rec.OSHash,
		MD5:             rec.MD5,
		Quality:         domain.Quality(rec.Quality),
		Resolution:      rec.Resolution,
		Codec:           rec.Codec,
		Container:       rec.Container,
		MatchConfidence: mc,
		AddedAt:         strToTime(rec.AddedAt),
	}
}

func (r *mediaFileRepo) Save(_ context.Context, mf *domain.MediaFile) error {
	if mf.ID == "" {
		mf.ID = newID()
	}
	if mf.AddedAt.IsZero() {
		mf.AddedAt = strToTime(nowStr())
	}

	return r.db.Update(func(txn *badgerdb.Txn) error {
		// Clean up old index entries for this media file.
		if old, err := getJSON[mediaFileRecord](txn, kMF(mf.ID)); err == nil {
			if old.ItemID != "" {
				_ = txn.Delete(kMFI(old.ItemID))
			}
			if old.OSHash != "" {
				_ = txn.Delete(kMFH(old.OSHash))
			}
		}

		mc := mf.MatchConfidence
		if mc == "" {
			mc = domain.MatchNameMatched
		}
		rec := mediaFileRecord{
			ID:              mf.ID,
			ItemID:          mf.ItemID,
			Path:            mf.Path,
			Size:            mf.Size,
			OSHash:          mf.OSHash,
			MD5:             mf.MD5,
			Quality:         string(mf.Quality),
			Resolution:      mf.Resolution,
			Codec:           mf.Codec,
			Container:       mf.Container,
			MatchConfidence: string(mc),
			AddedAt:         timeToStr(mf.AddedAt),
		}
		if err := setJSON(txn, kMF(mf.ID), rec); err != nil {
			return err
		}

		if mf.ItemID != "" {
			if err := txn.Set(kMFI(mf.ItemID), []byte(mf.ID)); err != nil {
				return err
			}
		}
		if mf.OSHash != "" {
			if err := txn.Set(kMFH(mf.OSHash), []byte(mf.ID)); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *mediaFileRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if old, err := getJSON[mediaFileRecord](txn, kMF(id)); err == nil {
			if old.ItemID != "" {
				_ = txn.Delete(kMFI(old.ItemID))
			}
			if old.OSHash != "" {
				_ = txn.Delete(kMFH(old.OSHash))
			}
		}
		return txn.Delete(kMF(id))
	})
}
