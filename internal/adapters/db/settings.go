package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/ports"
)

type settingsRepo struct {
	db *sql.DB
}

// NewSettingsRepo returns a SettingsRepository backed by SQLite.
func NewSettingsRepo(db *sql.DB) ports.SettingsRepository {
	return &settingsRepo{db: db}
}

func (r *settingsRepo) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errs.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, nil
}

func (r *settingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

var _ ports.SettingsRepository = (*settingsRepo)(nil)
