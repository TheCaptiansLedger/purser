package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
)

type unmatchedFileRepo struct {
	db *sql.DB
}

// NewUnmatchedFileRepo returns an UnmatchedFileRepository backed by SQL (SQLite or PostgreSQL).
func NewUnmatchedFileRepo(db *sql.DB) ports.UnmatchedFileRepository {
	return &unmatchedFileRepo{db: db}
}

var _ ports.UnmatchedFileRepository = (*unmatchedFileRepo)(nil)

const unmatchedFileSelectCols = `
	id, path, size, content_type, fingerprint, candidates, status, discovered_at, duplicate_of, thumbnail_path
`

func scanUnmatchedFile(row interface{ Scan(...any) error }) (*domain.UnmatchedFile, error) {
	var (
		f             domain.UnmatchedFile
		contentType   string
		fingerprint   string
		candidates    string
		status        string
		discoveredAt  string
		duplicateOf   string
		thumbnailPath string
	)
	if err := row.Scan(
		&f.ID, &f.Path, &f.Size, &contentType,
		&fingerprint, &candidates, &status, &discoveredAt,
		&duplicateOf, &thumbnailPath,
	); err != nil {
		return nil, err
	}
	f.ContentType = domain.ContentType(contentType)
	f.Status = domain.UnmatchedStatus(status)
	if f.Status == "" {
		f.Status = domain.UnmatchedPending
	}
	f.DiscoveredAt = strToTime(discoveredAt)
	f.Fingerprint = unmarshalFingerprint(fingerprint)
	f.Candidates = unmarshalCandidates(candidates)
	f.DuplicateOf = duplicateOf
	f.ThumbnailPath = thumbnailPath
	return &f, nil
}

func (r *unmatchedFileRepo) List(ctx context.Context, f ports.UnmatchedFilter) ([]*domain.UnmatchedFile, error) {
	w := &whereClause{}
	if f.ContentType != "" {
		w.add("content_type = ?", string(f.ContentType))
	}
	if f.Status != "" {
		w.add("status = ?", string(f.Status))
	}
	if f.Path != "" {
		w.add("path = ?", f.Path)
	}
	where, args := w.build()
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+unmatchedFileSelectCols+`FROM unmatched_files WHERE `+where+` ORDER BY discovered_at DESC`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list unmatched files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var files []*domain.UnmatchedFile
	for rows.Next() {
		uf, err := scanUnmatchedFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan unmatched file: %w", err)
		}
		files = append(files, uf)
	}
	return files, rows.Err()
}

func (r *unmatchedFileRepo) Get(ctx context.Context, id string) (*domain.UnmatchedFile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+unmatchedFileSelectCols+`FROM unmatched_files WHERE id = ?`, id)
	uf, err := scanUnmatchedFile(row)
	if err != nil {
		return nil, fmt.Errorf("get unmatched file %s: %w", id, mapNotFound(err))
	}
	return uf, nil
}

func (r *unmatchedFileRepo) Save(ctx context.Context, f *domain.UnmatchedFile) error {
	if f.ID == "" {
		f.ID = newID()
	}
	if f.DiscoveredAt.IsZero() {
		f.DiscoveredAt = strToTime(nowStr())
	}
	if f.Status == "" {
		f.Status = domain.UnmatchedPending
	}

	fp := marshalFingerprint(f.Fingerprint)
	cs := marshalCandidates(f.Candidates)

	var exists bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM unmatched_files WHERE id = ?`, f.ID,
	).Scan(&exists); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check unmatched file %s: %w", f.ID, err)
	}

	if !exists {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO unmatched_files(id, path, size, content_type, fingerprint, candidates, status, discovered_at, duplicate_of, thumbnail_path)
			VALUES(?,?,?,?,?,?,?,?,?,?)`,
			f.ID, f.Path, f.Size, string(f.ContentType),
			fp, cs, string(f.Status), timeToStr(f.DiscoveredAt),
			f.DuplicateOf, f.ThumbnailPath,
		)
		if err != nil {
			return fmt.Errorf("insert unmatched file: %w", err)
		}
		return nil
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE unmatched_files
		SET path=?, size=?, content_type=?, fingerprint=?, candidates=?, status=?, duplicate_of=?, thumbnail_path=?
		WHERE id=?`,
		f.Path, f.Size, string(f.ContentType), fp, cs, string(f.Status),
		f.DuplicateOf, f.ThumbnailPath, f.ID,
	)
	if err != nil {
		return fmt.Errorf("update unmatched file: %w", err)
	}
	return nil
}

func (r *unmatchedFileRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM unmatched_files WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete unmatched file %s: %w", id, err)
	}
	return nil
}

// ── JSON blob helpers ─────────────────────────────────────────────────────────

type dbFingerprintRecord struct {
	OSHash       string            `json:"oshash,omitempty"`
	PHash        string            `json:"phash,omitempty"`
	AcoustID     string            `json:"acoustid,omitempty"`
	EmbeddedTags map[string]string `json:"embedded_tags,omitempty"`
	ISBN         string            `json:"isbn,omitempty"`
}

type dbCandidateRecord struct {
	ItemID       string               `json:"item_id,omitempty"`
	ExternalItem *domain.ExternalItem `json:"external_item,omitempty"`
	Confidence   float64              `json:"confidence"`
	Source       string               `json:"source"`
}

func marshalFingerprint(fp *domain.Fingerprint) string {
	if fp == nil {
		return "{}"
	}
	rec := dbFingerprintRecord{
		OSHash:       fp.OSHash,
		PHash:        fp.PHash,
		AcoustID:     fp.AcoustID,
		EmbeddedTags: fp.EmbeddedTags,
		ISBN:         fp.ISBN,
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

func unmarshalFingerprint(s string) *domain.Fingerprint {
	if s == "" || s == "{}" {
		return nil
	}
	var rec dbFingerprintRecord
	if err := json.Unmarshal([]byte(s), &rec); err != nil {
		return nil
	}
	return &domain.Fingerprint{
		OSHash:       rec.OSHash,
		PHash:        rec.PHash,
		AcoustID:     rec.AcoustID,
		EmbeddedTags: rec.EmbeddedTags,
		ISBN:         rec.ISBN,
	}
}

func marshalCandidates(cs []domain.MatchCandidate) string {
	if len(cs) == 0 {
		return "[]"
	}
	recs := make([]dbCandidateRecord, 0, len(cs))
	for _, c := range cs {
		rec := dbCandidateRecord{
			ExternalItem: c.ExternalItem,
			Confidence:   c.Confidence,
			Source:       c.Source,
		}
		if c.Item != nil {
			rec.ItemID = c.Item.ID
		}
		recs = append(recs, rec)
	}
	b, _ := json.Marshal(recs)
	return string(b)
}

func unmarshalCandidates(s string) []domain.MatchCandidate {
	if s == "" || s == "[]" {
		return nil
	}
	var recs []dbCandidateRecord
	if err := json.Unmarshal([]byte(s), &recs); err != nil {
		return nil
	}
	cs := make([]domain.MatchCandidate, 0, len(recs))
	for _, r := range recs {
		c := domain.MatchCandidate{
			ExternalItem: r.ExternalItem,
			Confidence:   r.Confidence,
			Source:       r.Source,
		}
		if r.ItemID != "" {
			c.Item = &domain.Item{ID: r.ItemID}
		}
		cs = append(cs, c)
	}
	return cs
}
