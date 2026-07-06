package db

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
)

type musicReleaseRepo struct{ db *sql.DB }

// NewMusicReleaseRepo returns a MusicReleaseRepository backed by SQL.
func NewMusicReleaseRepo(db *sql.DB) ports.MusicReleaseRepository {
	return &musicReleaseRepo{db: db}
}

var _ ports.MusicReleaseRepository = (*musicReleaseRepo)(nil)

const musicReleaseSelectCols = `
	id, group_id, library_entry_id, title, country, date, label, catalog_number,
	barcode, format, medium_count, track_count, is_default, monitored, status,
	cover_path, added_at, updated_at
`

func scanMusicRelease(row interface{ Scan(...any) error }) (*domain.MusicRelease, error) {
	var (
		r                    domain.MusicRelease
		isDefault, monitored int
		date, addedAt, updAt string
		status               string
	)
	if err := row.Scan(
		&r.ID, &r.GroupID, &r.LibraryEntryID, &r.Title, &r.Country,
		&date, &r.Label, &r.CatalogNumber, &r.Barcode, &r.Format,
		&r.MediumCount, &r.TrackCount, &isDefault, &monitored, &status,
		&r.CoverPath, &addedAt, &updAt,
	); err != nil {
		return nil, err
	}
	r.IsDefault = intToBool(isDefault)
	r.Monitored = intToBool(monitored)
	r.Status = domain.ReleaseStatus(status)
	r.Date = strToDate(date)
	r.AddedAt = strToTime(addedAt)
	r.UpdatedAt = strToTime(updAt)
	return &r, nil
}

func (r *musicReleaseRepo) Get(ctx context.Context, id string) (*domain.MusicRelease, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+musicReleaseSelectCols+`FROM music_releases WHERE id = ?`, id)
	rel, err := scanMusicRelease(row)
	if err != nil {
		return nil, fmt.Errorf("get music release %s: %w", id, mapNotFound(err))
	}
	if err := r.attachExternalIDs(ctx, []*domain.MusicRelease{rel}); err != nil {
		return nil, fmt.Errorf("load external ids for music release %s: %w", id, err)
	}
	return rel, nil
}

func (r *musicReleaseRepo) GetByMBID(ctx context.Context, mbid string) (*domain.MusicRelease, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+musicReleaseSelectCols+`FROM music_releases mr
		 JOIN external_ids ei ON ei.entity_type = 'music_release' AND ei.entity_id = mr.id
		 WHERE ei.source = ? AND ei.value = ?`,
		string(domain.SourceMusicBrainz), mbid)
	rel, err := scanMusicRelease(row)
	if err != nil {
		return nil, fmt.Errorf("get music release by mbid %s: %w", mbid, mapNotFound(err))
	}
	if err := r.attachExternalIDs(ctx, []*domain.MusicRelease{rel}); err != nil {
		return nil, fmt.Errorf("load external ids for music release mbid %s: %w", mbid, err)
	}
	return rel, nil
}

func (r *musicReleaseRepo) GetByBarcode(ctx context.Context, barcode string) (*domain.MusicRelease, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+musicReleaseSelectCols+`FROM music_releases WHERE barcode = ?`, barcode)
	rel, err := scanMusicRelease(row)
	if err != nil {
		return nil, fmt.Errorf("get music release by barcode %s: %w", barcode, mapNotFound(err))
	}
	if err := r.attachExternalIDs(ctx, []*domain.MusicRelease{rel}); err != nil {
		return nil, fmt.Errorf("load external ids for music release barcode %s: %w", barcode, err)
	}
	return rel, nil
}

func (r *musicReleaseRepo) ListByGroup(ctx context.Context, groupID string) ([]*domain.MusicRelease, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+musicReleaseSelectCols+`FROM music_releases WHERE group_id = ? ORDER BY date, title`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("list music releases by group %s: %w", groupID, err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanAndAttach(ctx, rows, fmt.Sprintf("list music releases by group %s", groupID))
}

func (r *musicReleaseRepo) ListByEntry(ctx context.Context, entryID string) ([]*domain.MusicRelease, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+musicReleaseSelectCols+`FROM music_releases WHERE library_entry_id = ? ORDER BY date, title`,
		entryID)
	if err != nil {
		return nil, fmt.Errorf("list music releases by entry %s: %w", entryID, err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanAndAttach(ctx, rows, fmt.Sprintf("list music releases by entry %s", entryID))
}

func (r *musicReleaseRepo) scanAndAttach(ctx context.Context, rows *sql.Rows, errCtx string) ([]*domain.MusicRelease, error) {
	var results []*domain.MusicRelease
	for rows.Next() {
		rel, err := scanMusicRelease(rows)
		if err != nil {
			return nil, fmt.Errorf("%s scan: %w", errCtx, err)
		}
		results = append(results, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", errCtx, err)
	}
	if err := r.attachExternalIDs(ctx, results); err != nil {
		return nil, fmt.Errorf("%s external ids: %w", errCtx, err)
	}
	return results, nil
}

// ListTracksByRelease finds items whose metadata contains the given release_id.
// SQLite's json_extract is used to avoid a full item scan.
func (r *musicReleaseRepo) ListTracksByRelease(ctx context.Context, releaseID string) ([]*domain.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+itemSelectCols+`FROM items WHERE json_extract(metadata, '$.release_id') = ?`, //nolint:gosec // releaseID is an internal UUID, not user input
		releaseID)
	if err != nil {
		return nil, fmt.Errorf("list tracks by release %s: %w", releaseID, err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan track for release %s: %w", releaseID, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *musicReleaseRepo) Save(ctx context.Context, rel *domain.MusicRelease) error {
	if rel.ID == "" {
		rel.ID = newID()
	}
	rel.ApplyDefaults()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save music release: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO music_releases(
			id, group_id, library_entry_id, title, country, date, label,
			catalog_number, barcode, format, medium_count, track_count,
			is_default, monitored, status, cover_path, added_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			group_id         = excluded.group_id,
			library_entry_id = excluded.library_entry_id,
			title            = excluded.title,
			country          = excluded.country,
			date             = excluded.date,
			label            = excluded.label,
			catalog_number   = excluded.catalog_number,
			barcode          = excluded.barcode,
			format           = excluded.format,
			medium_count     = excluded.medium_count,
			track_count      = excluded.track_count,
			is_default       = excluded.is_default,
			monitored        = excluded.monitored,
			status           = excluded.status,
			cover_path       = excluded.cover_path,
			updated_at       = excluded.updated_at`,
		rel.ID, rel.GroupID, rel.LibraryEntryID, rel.Title, rel.Country,
		dateToStr(rel.Date), rel.Label, rel.CatalogNumber, rel.Barcode, rel.Format,
		rel.MediumCount, rel.TrackCount, boolToInt(rel.IsDefault), boolToInt(rel.Monitored),
		string(rel.Status), rel.CoverPath, timeToStr(rel.AddedAt), timeToStr(rel.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("save music release: upsert: %w", err)
	}
	if err := saveExternalIDs(ctx, tx, "music_release", rel.ID, rel.ExternalIDs); err != nil {
		return fmt.Errorf("save music release: external ids: %w", err)
	}
	return tx.Commit()
}

func (r *musicReleaseRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete music release: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM external_ids WHERE entity_type = 'music_release' AND entity_id = ?`, id); err != nil {
		return fmt.Errorf("delete music release external ids %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM music_releases WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete music release %s: %w", id, err)
	}
	return tx.Commit()
}

func (r *musicReleaseRepo) attachExternalIDs(ctx context.Context, releases []*domain.MusicRelease) error {
	return attachExternalIDsBatch(ctx, r.db, "music_release", releases,
		func(rel *domain.MusicRelease) string { return rel.ID },
		func(rel *domain.MusicRelease, ids []domain.ExternalID) { rel.ExternalIDs = ids },
	)
}
