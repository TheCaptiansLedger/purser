package db

import (
	"context"
	"database/sql"
	"fmt"

	"purser/internal/domain"
	"purser/internal/ports"
)

type mediaFileRepo struct {
	db *sql.DB
}

// NewMediaFileRepo returns a MediaFileRepository backed by SQLite.
func NewMediaFileRepo(db *sql.DB) ports.MediaFileRepository {
	return &mediaFileRepo{db: db}
}

const mediaFileSelectCols = `
	id, item_id, path, size, oshash, md5, quality, resolution, codec, container, added_at
`

func scanMediaFile(row interface{ Scan(...any) error }) (*domain.MediaFile, error) {
	var (
		mf      domain.MediaFile
		quality string
		addedAt string
	)
	if err := row.Scan(
		&mf.ID, &mf.ItemID, &mf.Path, &mf.Size,
		&mf.OSHash, &mf.MD5, &quality, &mf.Resolution,
		&mf.Codec, &mf.Container, &addedAt,
	); err != nil {
		return nil, err
	}
	mf.Quality = domain.Quality(quality)
	mf.AddedAt = strToTime(addedAt)
	return &mf, nil
}

func (r *mediaFileRepo) GetByItemID(ctx context.Context, itemID string) (*domain.MediaFile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+mediaFileSelectCols+`FROM media_files WHERE item_id = ?`, itemID)
	mf, err := scanMediaFile(row)
	if err != nil {
		return nil, fmt.Errorf("get media file for item %s: %w", itemID, err)
	}
	return mf, nil
}

func (r *mediaFileRepo) GetByOSHash(ctx context.Context, hash string) (*domain.MediaFile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+mediaFileSelectCols+`FROM media_files WHERE oshash = ?`, hash)
	mf, err := scanMediaFile(row)
	if err != nil {
		return nil, fmt.Errorf("get media file by oshash %s: %w", hash, err)
	}
	return mf, nil
}

func (r *mediaFileRepo) Save(ctx context.Context, mf *domain.MediaFile) error {
	if mf.ID == "" {
		mf.ID = newID()
	}
	if mf.AddedAt.IsZero() {
		mf.AddedAt = strToTime(nowStr())
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO media_files(
			id, item_id, path, size, oshash, md5,
			quality, resolution, codec, container, added_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			item_id    = excluded.item_id,
			path       = excluded.path,
			size       = excluded.size,
			oshash     = excluded.oshash,
			md5        = excluded.md5,
			quality    = excluded.quality,
			resolution = excluded.resolution,
			codec      = excluded.codec,
			container  = excluded.container`,
		mf.ID, mf.ItemID, mf.Path, mf.Size,
		mf.OSHash, mf.MD5, string(mf.Quality), mf.Resolution,
		mf.Codec, mf.Container, timeToStr(mf.AddedAt),
	)
	if err != nil {
		return fmt.Errorf("save media file: %w", err)
	}
	return nil
}

func (r *mediaFileRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM media_files WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete media file %s: %w", id, err)
	}
	return nil
}

var _ ports.MediaFileRepository = (*mediaFileRepo)(nil)
