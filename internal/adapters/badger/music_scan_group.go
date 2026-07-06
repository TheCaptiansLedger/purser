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

type musicScanGroupRepo struct{ db *badgerdb.DB }

// NewMusicScanGroupRepo returns a MusicScanGroupRepository backed by BadgerDB.
func NewMusicScanGroupRepo(db *badgerdb.DB) ports.MusicScanGroupRepository {
	return &musicScanGroupRepo{db: db}
}

var _ ports.MusicScanGroupRepository = (*musicScanGroupRepo)(nil)

func (r *musicScanGroupRepo) Get(_ context.Context, id string) (*domain.MusicScanGroup, error) {
	var g *domain.MusicScanGroup
	err := r.db.View(func(txn *badgerdb.Txn) error {
		rec, err := getJSON[musicScanGroupRecord](txn, kMSG(id))
		if err != nil {
			return err
		}
		g = musicScanGroupFromRecord(rec)
		return nil
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get music scan group %s: %w", id, err)
	}
	return g, nil
}

func (r *musicScanGroupRepo) List(_ context.Context, status domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error) {
	var results []*domain.MusicScanGroup
	pfx := pfxMSGStatus(string(status))
	err := r.db.View(func(txn *badgerdb.Txn) error {
		var ids []string
		iterPrefix(txn, pfx, func(key []byte) bool {
			ids = append(ids, suffixAfter(key, pfx))
			return true
		})
		for _, id := range ids {
			rec, err := getJSON[musicScanGroupRecord](txn, kMSG(id))
			if err != nil {
				continue
			}
			results = append(results, musicScanGroupFromRecord(rec))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list music scan groups by status %s: %w", status, err)
	}
	return results, nil
}

func (r *musicScanGroupRepo) Save(_ context.Context, g *domain.MusicScanGroup) error {
	if g.ID == "" {
		g.ID = newID()
	}
	if g.DiscoveredAt.IsZero() {
		g.DiscoveredAt = strToTime(nowStr())
	}
	if g.Status == "" {
		g.Status = domain.UnmatchedPending
	}

	return r.db.Update(func(txn *badgerdb.Txn) error {
		if old, err := getJSON[musicScanGroupRecord](txn, kMSG(g.ID)); err == nil {
			_ = txn.Delete(kMSGStatus(old.Status, g.ID))
		}
		rec := musicScanGroupToRecord(g)
		if err := setJSON(txn, kMSG(g.ID), rec); err != nil {
			return err
		}
		return txn.Set(kMSGStatus(string(g.Status), g.ID), []byte(g.ID))
	})
}

func (r *musicScanGroupRepo) Delete(_ context.Context, id string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if old, err := getJSON[musicScanGroupRecord](txn, kMSG(id)); err == nil {
			_ = txn.Delete(kMSGStatus(old.Status, id))
		}
		if err := txn.Delete(kMSG(id)); err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
			return fmt.Errorf("delete music scan group %s: %w", id, err)
		}
		return nil
	})
}

func musicScanGroupFromRecord(rec *musicScanGroupRecord) *domain.MusicScanGroup {
	return &domain.MusicScanGroup{
		ID:           rec.ID,
		FolderPath:   rec.FolderPath,
		TotalTracks:  rec.TotalTracks,
		TotalDiscs:   rec.TotalDiscs,
		Status:       domain.UnmatchedStatus(rec.Status),
		Candidates:   rec.Candidates,
		DiscoveredAt: strToTime(rec.DiscoveredAt),
	}
}

func musicScanGroupToRecord(g *domain.MusicScanGroup) musicScanGroupRecord {
	return musicScanGroupRecord{
		ID:           g.ID,
		FolderPath:   g.FolderPath,
		TotalTracks:  g.TotalTracks,
		TotalDiscs:   g.TotalDiscs,
		Status:       string(g.Status),
		Candidates:   g.Candidates,
		DiscoveredAt: timeToStr(g.DiscoveredAt),
	}
}
