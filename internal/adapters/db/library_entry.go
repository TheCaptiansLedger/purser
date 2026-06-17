package db

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
)

type libraryEntryRepo struct {
	db *sql.DB
}

// NewLibraryEntryRepo returns a LibraryEntryRepository backed by SQLite.
func NewLibraryEntryRepo(db *sql.DB) ports.LibraryEntryRepository {
	return &libraryEntryRepo{db: db}
}

const entrySelectCols = `
	id, content_type, kind, name, sort_name, overview,
	COALESCE(parent_id, ''), monitored, monitor_mode, status,
	quality_profile_id, metadata_profile_id, path, image_path, metadata,
	added_at, updated_at
`

func scanEntry(row interface{ Scan(...any) error }) (*domain.LibraryEntry, error) {
	var (
		e                            domain.LibraryEntry
		contentType, kind            string
		monitorMode, status          string
		monitored                    int
		metadata, addedAt, updatedAt string
	)
	if err := row.Scan(
		&e.ID, &contentType, &kind, &e.Name, &e.SortName, &e.Overview,
		&e.ParentID, &monitored, &monitorMode, &status,
		&e.QualityProfileID, &e.MetadataProfileID, &e.Path, &e.ImagePath,
		&metadata, &addedAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	e.ContentType = domain.ContentType(contentType)
	e.Kind = domain.Kind(kind)
	e.MonitorMode = domain.MonitorMode(monitorMode)
	e.Status = domain.EntryStatus(status)
	e.Monitored = intToBool(monitored)
	e.Metadata = unmarshalMeta(metadata)
	e.AddedAt = strToTime(addedAt)
	e.UpdatedAt = strToTime(updatedAt)
	return &e, nil
}

func (r *libraryEntryRepo) Get(ctx context.Context, id string) (*domain.LibraryEntry, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+entrySelectCols+`FROM library_entries WHERE id = ?`, id)
	e, err := scanEntry(row)
	if err != nil {
		return nil, fmt.Errorf("get library entry %s: %w", id, err)
	}

	ids, err := loadExternalIDs(ctx, r.db, "library_entry", id)
	if err != nil {
		return nil, fmt.Errorf("load external ids for %s: %w", id, err)
	}
	e.ExternalIDs = ids

	tags, err := loadEntryTags(ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("load tags for %s: %w", id, err)
	}
	e.Tags = tags

	return e, nil
}

func (r *libraryEntryRepo) List(ctx context.Context, f ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	w := &whereClause{}

	if f.ContentType != "" {
		w.add("content_type = ?", string(f.ContentType))
	}
	if f.Kind != "" {
		w.add("kind = ?", string(f.Kind))
	}
	if f.ParentID != "" {
		w.add("parent_id = ?", f.ParentID)
	}
	if f.Monitored != nil {
		w.add("monitored = ?", boolToInt(*f.Monitored))
	}
	if f.Search != "" {
		w.add("name LIKE ?", "%"+f.Search+"%")
	}

	where, args := w.build()

	var total int
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM library_entries WHERE `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count library entries: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	queryArgs := append(args, limit, f.Offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+entrySelectCols+`FROM library_entries WHERE `+where+
			` ORDER BY sort_name, name LIMIT ? OFFSET ?`,
		queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list library entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []*domain.LibraryEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

func (r *libraryEntryRepo) Save(ctx context.Context, e *domain.LibraryEntry) error {
	if e.ID == "" {
		e.ID = newID()
	}
	now := nowStr()
	if e.AddedAt.IsZero() {
		e.AddedAt = strToTime(now)
	}
	e.UpdatedAt = strToTime(now)

	var parentID *string
	if e.ParentID != "" {
		parentID = &e.ParentID
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save library entry: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx, `
		INSERT INTO library_entries(
			id, content_type, kind, name, sort_name, overview, parent_id,
			monitored, monitor_mode, status, quality_profile_id, metadata_profile_id,
			path, image_path, metadata, added_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			content_type        = excluded.content_type,
			kind                = excluded.kind,
			name                = excluded.name,
			sort_name           = excluded.sort_name,
			overview            = excluded.overview,
			parent_id           = excluded.parent_id,
			monitored           = excluded.monitored,
			monitor_mode        = excluded.monitor_mode,
			status              = excluded.status,
			quality_profile_id  = excluded.quality_profile_id,
			metadata_profile_id = excluded.metadata_profile_id,
			path                = excluded.path,
			image_path          = excluded.image_path,
			metadata            = excluded.metadata,
			updated_at          = excluded.updated_at`,
		e.ID, string(e.ContentType), string(e.Kind), e.Name, e.SortName, e.Overview, parentID,
		boolToInt(e.Monitored), string(e.MonitorMode), string(e.Status),
		e.QualityProfileID, e.MetadataProfileID, e.Path, e.ImagePath,
		marshalMeta(e.Metadata), timeToStr(e.AddedAt), now,
	); err != nil {
		return fmt.Errorf("upsert library entry: %w", err)
	}

	if err := saveExternalIDs(ctx, tx, "library_entry", e.ID, e.ExternalIDs); err != nil {
		return fmt.Errorf("save external ids: %w", err)
	}
	if err := saveEntryTags(ctx, tx, e.ID, e.Tags); err != nil {
		return fmt.Errorf("save tags: %w", err)
	}

	return tx.Commit()
}

func (r *libraryEntryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM library_entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete library entry %s: %w", id, err)
	}
	return nil
}

// ensure interface is satisfied at compile time
var _ ports.LibraryEntryRepository = (*libraryEntryRepo)(nil)

// listPlaceholders returns n comma-separated ? placeholders for SQL IN clauses.
func listPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
