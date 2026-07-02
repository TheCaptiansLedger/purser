package badger

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/ports"

	badgerdb "github.com/dgraph-io/badger/v4"
)

type externalIDRepo struct {
	db *badgerdb.DB
}

// NewExternalIDRepo returns an ExternalIDRepository backed by BadgerDB.
func NewExternalIDRepo(db *badgerdb.DB) ports.ExternalIDRepository {
	return &externalIDRepo{db: db}
}

var _ ports.ExternalIDRepository = (*externalIDRepo)(nil)

func (r *externalIDRepo) FindEntity(_ context.Context, entityType, source, value string) (string, error) {
	var entityID string
	err := r.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(kEID(entityType, source, value))
		if err != nil {
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			entityID = string(val)
			return nil
		})
	})
	if err != nil {
		if errs.IsNotFound(err) {
			return "", errs.ErrNotFound
		}
		return "", fmt.Errorf("find entity %s/%s/%s: %w", entityType, source, value, err)
	}
	return entityID, nil
}
