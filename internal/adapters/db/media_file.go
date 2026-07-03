package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
)

type mediaFileRepo struct {
	db *sql.DB
}

// NewMediaFileRepo returns a MediaFileRepository backed by SQL (SQLite or PostgreSQL).
func NewMediaFileRepo(db *sql.DB) ports.MediaFileRepository {
	return &mediaFileRepo{db: db}
}

const mediaFileSelectCols = `
	id, item_id, path, size, oshash, md5, quality, resolution, codec, container, added_at, match_confidence
`

func scanMediaFile(row interface{ Scan(...any) error }) (*domain.MediaFile, error) {
	var (
		mf              domain.MediaFile
		quality         string
		addedAt         string
		matchConfidence string
	)
	if err := row.Scan(
		&mf.ID, &mf.ItemID, &mf.Path, &mf.Size,
		&mf.OSHash, &mf.MD5, &quality, &mf.Resolution,
		&mf.Codec, &mf.Container, &addedAt, &matchConfidence,
	); err != nil {
		return nil, err
	}
	mf.Quality = domain.Quality(quality)
	mf.AddedAt = strToTime(addedAt)
	mc := domain.MatchConfidence(matchConfidence)
	if mc == "" {
		mc = domain.MatchNameMatched
	}
	mf.MatchConfidence = mc
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

func (r *mediaFileRepo) GetByPath(ctx context.Context, path string) (*domain.MediaFile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+mediaFileSelectCols+`FROM media_files WHERE path = ?`, path)
	mf, err := scanMediaFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get media file by path %s: %w", path, err)
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

	mc := mf.MatchConfidence
	if mc == "" {
		mc = domain.MatchNameMatched
	}

	_, err := r.db.ExecContext(
		ctx, `
		INSERT INTO media_files(
			id, item_id, path, size, oshash, md5,
			quality, resolution, codec, container, added_at, match_confidence
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			item_id          = excluded.item_id,
			path             = excluded.path,
			size             = excluded.size,
			oshash           = excluded.oshash,
			md5              = excluded.md5,
			quality          = excluded.quality,
			resolution       = excluded.resolution,
			codec            = excluded.codec,
			container        = excluded.container,
			match_confidence = excluded.match_confidence`,
		mf.ID, mf.ItemID, mf.Path, mf.Size,
		mf.OSHash, mf.MD5, string(mf.Quality), mf.Resolution,
		mf.Codec, mf.Container, timeToStr(mf.AddedAt), string(mc),
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
