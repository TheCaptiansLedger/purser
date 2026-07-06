package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
)

type musicScanGroupRepo struct{ db *sql.DB }

// NewMusicScanGroupRepo returns a MusicScanGroupRepository backed by SQL.
func NewMusicScanGroupRepo(db *sql.DB) ports.MusicScanGroupRepository {
	return &musicScanGroupRepo{db: db}
}

var _ ports.MusicScanGroupRepository = (*musicScanGroupRepo)(nil)

func marshalMusicCandidates(cs []domain.MusicReleaseCandidate) string {
	if len(cs) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(cs)
	return string(b)
}

func unmarshalMusicCandidates(s string) []domain.MusicReleaseCandidate {
	if s == "" || s == "[]" {
		return nil
	}
	var cs []domain.MusicReleaseCandidate
	_ = json.Unmarshal([]byte(s), &cs)
	return cs
}

func scanMusicScanGroup(row interface{ Scan(...any) error }) (*domain.MusicScanGroup, error) {
	var (
		g            domain.MusicScanGroup
		status       string
		candidates   string
		discoveredAt string
	)
	if err := row.Scan(
		&g.ID, &g.FolderPath, &g.TotalTracks, &g.TotalDiscs,
		&status, &candidates, &discoveredAt,
	); err != nil {
		return nil, err
	}
	g.Status = domain.UnmatchedStatus(status)
	g.Candidates = unmarshalMusicCandidates(candidates)
	g.DiscoveredAt = strToTime(discoveredAt)
	return &g, nil
}

func (r *musicScanGroupRepo) Get(ctx context.Context, id string) (*domain.MusicScanGroup, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, folder_path, total_tracks, total_discs, status, candidates, discovered_at
		 FROM music_scan_groups WHERE id = ?`, id)
	g, err := scanMusicScanGroup(row)
	if err != nil {
		return nil, fmt.Errorf("get music scan group %s: %w", id, mapNotFound(err))
	}
	return g, nil
}

func (r *musicScanGroupRepo) List(ctx context.Context, status domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, folder_path, total_tracks, total_discs, status, candidates, discovered_at
		 FROM music_scan_groups WHERE status = ? ORDER BY discovered_at DESC`,
		string(status))
	if err != nil {
		return nil, fmt.Errorf("list music scan groups by status %s: %w", status, err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.MusicScanGroup
	for rows.Next() {
		g, err := scanMusicScanGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan music scan group: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

func (r *musicScanGroupRepo) Save(ctx context.Context, g *domain.MusicScanGroup) error {
	if g.ID == "" {
		g.ID = newID()
	}
	if g.DiscoveredAt.IsZero() {
		g.DiscoveredAt = strToTime(nowStr())
	}
	if g.Status == "" {
		g.Status = domain.UnmatchedPending
	}

	candidates := marshalMusicCandidates(g.Candidates)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO music_scan_groups(id, folder_path, total_tracks, total_discs, status, candidates, discovered_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			folder_path   = excluded.folder_path,
			total_tracks  = excluded.total_tracks,
			total_discs   = excluded.total_discs,
			status        = excluded.status,
			candidates    = excluded.candidates`,
		g.ID, g.FolderPath, g.TotalTracks, g.TotalDiscs,
		string(g.Status), candidates, timeToStr(g.DiscoveredAt),
	)
	if err != nil {
		return fmt.Errorf("save music scan group: %w", err)
	}
	return nil
}

func (r *musicScanGroupRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM music_scan_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete music scan group %s: %w", id, err)
	}
	return nil
}
