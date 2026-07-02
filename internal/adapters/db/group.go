package db

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
)

type groupRepo struct {
	db *sql.DB
}

// NewGroupRepo returns a GroupRepository backed by SQLite.
func NewGroupRepo(db *sql.DB) ports.GroupRepository {
	return &groupRepo{db: db}
}

const groupSelectCols = `
	id, library_entry_id, title, sort_name, number, year, overview,
	monitored, monitor_mode, metadata, locked_fields, cover_path
`

func scanGroup(row interface{ Scan(...any) error }) (*domain.Group, error) {
	var (
		g            domain.Group
		monitorMode  string
		monitored    int
		metadata     string
		lockedFields string
	)
	if err := row.Scan(
		&g.ID, &g.LibraryEntryID, &g.Title, &g.SortName, &g.Number, &g.Year, &g.Overview,
		&monitored, &monitorMode, &metadata, &lockedFields, &g.CoverPath,
	); err != nil {
		return nil, err
	}
	g.Monitored = intToBool(monitored)
	g.MonitorMode = domain.MonitorMode(monitorMode)
	g.Metadata = unmarshalMeta(metadata)
	g.LockedFields = unmarshalLockedFields(lockedFields)
	return &g, nil
}

func (r *groupRepo) Get(ctx context.Context, id string) (*domain.Group, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+groupSelectCols+`FROM groups WHERE id = ?`, id)
	g, err := scanGroup(row)
	if err != nil {
		return nil, fmt.Errorf("get group %s: %w", id, err)
	}

	ids, err := loadExternalIDs(ctx, r.db, "group", id)
	if err != nil {
		return nil, fmt.Errorf("load external ids for group %s: %w", id, err)
	}
	g.ExternalIDs = ids

	tags, err := loadGroupTags(ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("load tags for group %s: %w", id, err)
	}
	g.Tags = tags

	return g, nil
}

func (r *groupRepo) List(ctx context.Context, f ports.GroupFilter) ([]*domain.Group, error) {
	w := &whereClause{}

	if f.LibraryEntryID != "" {
		w.add("library_entry_id = ?", f.LibraryEntryID)
	}
	if f.Monitored != nil {
		w.add("monitored = ?", boolToInt(*f.Monitored))
	}
	if f.TagKey != "" || f.TagValue != "" {
		var conds []string
		var tagArgs []any
		if f.TagKey != "" {
			conds = append(conds, "t.key = ?")
			tagArgs = append(tagArgs, string(f.TagKey))
		}
		if f.TagValue != "" {
			conds = append(conds, "t.value = ?")
			tagArgs = append(tagArgs, f.TagValue)
		}
		sub := "SELECT gt.group_id FROM group_tags gt JOIN tags t ON t.id = gt.tag_id WHERE " + strings.Join(conds, " AND ")
		w.add("id IN ("+sub+")", tagArgs...) //nolint:gosec // sub contains only hardcoded column predicates with ? parameters
	}

	where, args := w.build()

	rows, err := r.db.QueryContext(ctx,
		`SELECT`+groupSelectCols+`FROM groups WHERE `+where+` ORDER BY number, title`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var groups []*domain.Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := attachExternalIDsBatch(ctx, r.db, "group", groups,
		func(g *domain.Group) string { return g.ID },
		func(g *domain.Group, ids []domain.ExternalID) { g.ExternalIDs = ids },
	); err != nil {
		return nil, fmt.Errorf("load external ids for groups: %w", err)
	}

	return groups, nil
}

func (r *groupRepo) Save(ctx context.Context, g *domain.Group) error {
	if g.ID == "" {
		g.ID = newID()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save group: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx, `
		INSERT INTO groups(
			id, library_entry_id, title, sort_name, number, year, overview,
			monitored, monitor_mode, metadata, locked_fields, cover_path
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			library_entry_id = excluded.library_entry_id,
			title            = excluded.title,
			sort_name        = excluded.sort_name,
			number           = excluded.number,
			year             = excluded.year,
			overview         = excluded.overview,
			monitored        = excluded.monitored,
			monitor_mode     = excluded.monitor_mode,
			metadata         = excluded.metadata,
			locked_fields    = excluded.locked_fields,
			cover_path       = excluded.cover_path`,
		g.ID, g.LibraryEntryID, g.Title, g.SortName, g.Number, g.Year, g.Overview,
		boolToInt(g.Monitored), string(g.MonitorMode), marshalMeta(g.Metadata),
		marshalLockedFields(g.LockedFields), g.CoverPath,
	); err != nil {
		return fmt.Errorf("upsert group: %w", err)
	}

	if err := saveExternalIDs(ctx, tx, "group", g.ID, g.ExternalIDs); err != nil {
		return fmt.Errorf("save external ids: %w", err)
	}

	return tx.Commit()
}

func (r *groupRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete group %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DELETE FROM group_tags   WHERE group_id = ?`,
		`DELETE FROM external_ids WHERE entity_type = 'group' AND entity_id = ?`,
		`DELETE FROM groups       WHERE id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, id); err != nil {
			return fmt.Errorf("delete group %s: %w", id, err)
		}
	}
	return tx.Commit()
}

func (r *groupRepo) DeleteByLibraryEntry(ctx context.Context, entryID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete groups for entry %s: %w", entryID, err)
	}
	defer func() { _ = tx.Rollback() }()

	sub := `(SELECT id FROM groups WHERE library_entry_id = ?)`
	stmts := []string{
		`DELETE FROM group_tags   WHERE group_id IN ` + sub,
		`DELETE FROM external_ids WHERE entity_type = 'group' AND entity_id IN ` + sub,
		`DELETE FROM groups       WHERE library_entry_id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, entryID); err != nil {
			return fmt.Errorf("delete groups for entry %s: %w", entryID, err)
		}
	}
	return tx.Commit()
}

func (r *groupRepo) DeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	var title, ct string
	if err := r.db.QueryRowContext(ctx, `
		SELECT g.title, le.content_type
		FROM groups g
		JOIN library_entries le ON le.id = g.library_entry_id
		WHERE g.id = ?`, id,
	).Scan(&title, &ct); err != nil {
		return nil, fmt.Errorf("deletion impact for group %s: %w", id, err)
	}
	contentType := domain.ContentType(ct)

	var itemCount int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE group_id = ?`, id).Scan(&itemCount)

	impact := &domain.DeletionImpact{Mode: domain.DeletionModeDestroy, Impacts: []domain.DeletionImpactRow{}}
	if itemCount > 0 {
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind: "item", Count: itemCount, Label: contentType.ItemLabel(),
		})
		impact.Summary = fmt.Sprintf("Deleting %s will permanently remove %d %s.", title, itemCount, contentType.ItemLabel())
	} else {
		impact.Summary = fmt.Sprintf("Deleting %s will permanently remove it from the library.", title)
	}
	return impact, nil
}

var _ ports.GroupRepository = (*groupRepo)(nil)
