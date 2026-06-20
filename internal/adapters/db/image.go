package db

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
)

type imageRepo struct {
	db             *sql.DB
	sourcePriority []string
}

// NewImageRepo returns an ImageRepository backed by SQLite.
// sourcePriority is an ordered list of source names; index 0 = highest priority.
func NewImageRepo(db *sql.DB, sourcePriority []string) ports.ImageRepository {
	return &imageRepo{db: db, sourcePriority: sourcePriority}
}

func (r *imageRepo) Upsert(ctx context.Context, images []domain.StoredImage) error {
	for _, img := range images {
		id := img.ID
		if id == "" {
			id = newID()
		}
		_, err := r.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO images
			 (id, entity_type, entity_id, image_type, url, source, width, height, added_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, img.EntityType, img.EntityID, string(img.ImageType),
			img.URL, img.Source, img.Width, img.Height, nowStr(),
		)
		if err != nil {
			return fmt.Errorf("upsert image: %w", err)
		}
	}
	return nil
}

func (r *imageRepo) List(ctx context.Context, entityType, entityID string, imageType *domain.ImageType) ([]domain.StoredImage, error) {
	w := &whereClause{}
	w.add("entity_type = ?", entityType)
	w.add("entity_id = ?", entityID)
	if imageType != nil {
		w.add("image_type = ?", string(*imageType))
	}
	where, args := w.build()

	q := `SELECT id, entity_type, entity_id, image_type, url, source, width, height, added_at FROM images WHERE ` + where + ` ORDER BY added_at` //nolint:gosec // whereClause uses only ? placeholders; no user data in the SQL string
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []domain.StoredImage
	for rows.Next() {
		img, err := scanStoredImage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *img)
	}
	return result, rows.Err()
}

func (r *imageRepo) GetSelection(ctx context.Context, entityType, entityID string, imageType domain.ImageType) (*domain.StoredImage, error) {
	var imageID *string
	err := r.db.QueryRowContext(ctx,
		`SELECT image_id FROM image_selections
		 WHERE entity_type = ? AND entity_id = ? AND image_type = ?`,
		entityType, entityID, string(imageType),
	).Scan(&imageID)

	switch {
	case err == sql.ErrNoRows:
		// no selection row — fall through to auto-select
	case err != nil:
		return nil, fmt.Errorf("get image selection: %w", err)
	case imageID != nil:
		return r.getByID(ctx, *imageID)
	default:
		// imageID == nil: row exists but image was deleted (ON DELETE SET NULL) — auto-select
	}

	return r.autoSelect(ctx, entityType, entityID, imageType)
}

func (r *imageRepo) SetSelection(ctx context.Context, entityType, entityID string, imageType domain.ImageType, imageID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO image_selections (entity_type, entity_id, image_type, image_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (entity_type, entity_id, image_type) DO UPDATE SET image_id = excluded.image_id`,
		entityType, entityID, string(imageType), imageID,
	)
	if err != nil {
		return fmt.Errorf("set image selection: %w", err)
	}
	return nil
}

func (r *imageRepo) ClearSelection(ctx context.Context, entityType, entityID string, imageType domain.ImageType) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM image_selections WHERE entity_type = ? AND entity_id = ? AND image_type = ?`,
		entityType, entityID, string(imageType),
	)
	if err != nil {
		return fmt.Errorf("clear image selection: %w", err)
	}
	return nil
}

func (r *imageRepo) autoSelect(ctx context.Context, entityType, entityID string, imageType domain.ImageType) (*domain.StoredImage, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, entity_type, entity_id, image_type, url, source, width, height, added_at
		 FROM images WHERE entity_type = ? AND entity_id = ? AND image_type = ?`,
		entityType, entityID, string(imageType))
	if err != nil {
		return nil, fmt.Errorf("auto-select images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []domain.StoredImage
	for rows.Next() {
		img, err := scanStoredImage(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, *img)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil //nolint:nilnil // no images for this slot is a valid state, not an error
	}

	priorityOf := func(source string) int {
		for i, s := range r.sourcePriority {
			if s == source {
				return i
			}
		}
		return len(r.sourcePriority)
	}

	best := &candidates[0]
	bestPri := priorityOf(candidates[0].Source)
	for i := 1; i < len(candidates); i++ {
		if p := priorityOf(candidates[i].Source); p < bestPri {
			best = &candidates[i]
			bestPri = p
		}
	}
	return best, nil
}

func (r *imageRepo) getByID(ctx context.Context, id string) (*domain.StoredImage, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, entity_type, entity_id, image_type, url, source, width, height, added_at
		 FROM images WHERE id = ?`, id)
	img, err := scanStoredImage(row)
	if err != nil {
		return nil, fmt.Errorf("get image %s: %w", id, err)
	}
	return img, nil
}

func scanStoredImage(row interface{ Scan(...any) error }) (*domain.StoredImage, error) {
	var img domain.StoredImage
	var imageType, addedAt string
	if err := row.Scan(
		&img.ID, &img.EntityType, &img.EntityID, &imageType,
		&img.URL, &img.Source, &img.Width, &img.Height, &addedAt,
	); err != nil {
		return nil, err
	}
	img.ImageType = domain.ImageType(imageType)
	img.AddedAt = strToTime(addedAt)
	return &img, nil
}

var _ ports.ImageRepository = (*imageRepo)(nil)
