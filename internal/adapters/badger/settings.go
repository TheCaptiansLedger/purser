package badger

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/ports"

	badgerdb "github.com/dgraph-io/badger/v4"
)

type settingsRepo struct {
	db *badgerdb.DB
}

// NewSettingsRepo returns a SettingsRepository backed by BadgerDB.
func NewSettingsRepo(db *badgerdb.DB) ports.SettingsRepository {
	return &settingsRepo{db: db}
}

var _ ports.SettingsRepository = (*settingsRepo)(nil)

func (r *settingsRepo) Get(_ context.Context, key string) (string, error) {
	var val string
	err := r.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kCFG(key))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		return item.Value(func(v []byte) error {
			val = string(v)
			return nil
		})
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return "", errs.ErrNotFound
		}
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return val, nil
}

func (r *settingsRepo) Set(_ context.Context, key, value string) error {
	return r.db.Update(func(txn *badgerdb.Txn) error {
		if err := txn.Set(kCFG(key), []byte(value)); err != nil {
			return fmt.Errorf("set setting %s: %w", key, err)
		}
		return nil
	})
}
