package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/ports"
)

type externalIDRepo struct {
	db *sql.DB
}

// NewExternalIDRepo returns an ExternalIDRepository backed by SQLite.
func NewExternalIDRepo(db *sql.DB) ports.ExternalIDRepository {
	return &externalIDRepo{db: db}
}

func (r *externalIDRepo) FindEntity(ctx context.Context, entityType, source, value string) (string, error) {
	var id string
	err := r.db.QueryRowContext(
		ctx,
		`SELECT entity_id FROM external_ids
		 WHERE entity_type = ? AND source = ? AND value = ?`,
		entityType, source, value,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errs.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find entity by external id: %w", err)
	}
	return id, nil
}

var _ ports.ExternalIDRepository = (*externalIDRepo)(nil)
