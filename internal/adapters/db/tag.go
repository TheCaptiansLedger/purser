package db

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
)

type tagRepo struct {
	db *sql.DB
}

// NewTagRepo returns a TagRepository backed by SQLite.
func NewTagRepo(db *sql.DB) ports.TagRepository {
	return &tagRepo{db: db}
}

func (r *tagRepo) Get(ctx context.Context, id string) (*domain.Tag, error) {
	var t domain.Tag
	var key, scope string
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT id, key, value, scope FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &key, &t.Value, &scope); err != nil {
		return nil, fmt.Errorf("get tag %s: %w", id, err)
	}
	t.Key = domain.TagKey(key)
	t.Scope = domain.TagScope(scope)
	return &t, nil
}

func (r *tagRepo) List(ctx context.Context, f ports.TagFilter) ([]*domain.Tag, error) {
	q := `SELECT DISTINCT t.id, t.key, t.value, t.scope FROM tags t`
	var args []any
	var conditions []string

	if f.GroupID != "" {
		q += ` JOIN group_tags gt ON gt.tag_id = t.id`
		conditions = append(conditions, `gt.group_id = ?`)
		args = append(args, f.GroupID)
	}

	if len(f.ContentTypes) > 0 {
		placeholders := make([]string, len(f.ContentTypes))
		ctArgs := make([]any, len(f.ContentTypes))
		for i, ct := range f.ContentTypes {
			placeholders[i] = "?"
			ctArgs[i] = string(ct)
		}
		in := strings.Join(placeholders, ",")
		conditions = append(conditions, fmt.Sprintf(`(
			EXISTS (SELECT 1 FROM item_tags it JOIN items i ON i.id = it.item_id WHERE it.tag_id = t.id AND i.content_type IN (%s))
			OR EXISTS (SELECT 1 FROM entry_tags et JOIN library_entries le ON le.id = et.library_entry_id WHERE et.tag_id = t.id AND le.content_type IN (%s))
			OR EXISTS (SELECT 1 FROM group_tags gct JOIN groups g ON g.id = gct.group_id JOIN library_entries le ON le.id = g.library_entry_id WHERE gct.tag_id = t.id AND le.content_type IN (%s))
		)`, in, in, in))
		args = append(args, ctArgs...)
		args = append(args, ctArgs...)
		args = append(args, ctArgs...)
	}

	if f.Scope != "" {
		conditions = append(conditions, `t.scope = ?`)
		args = append(args, string(f.Scope))
	}

	if f.Key != "" {
		conditions = append(conditions, `t.key = ?`)
		args = append(args, string(f.Key))
	}

	if f.Value != "" {
		conditions = append(conditions, `t.value = ?`)
		args = append(args, f.Value)
	}

	if len(conditions) > 0 {
		q += ` WHERE ` + strings.Join(conditions, ` AND `) //nolint:gosec // conditions contain only parameterized placeholders, no user input
	}
	q += ` ORDER BY t.value`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tags []*domain.Tag
	for rows.Next() {
		var t domain.Tag
		var key, sc string
		if err := rows.Scan(&t.ID, &key, &t.Value, &sc); err != nil {
			return nil, err
		}
		t.Key = domain.TagKey(key)
		t.Scope = domain.TagScope(sc)
		tags = append(tags, &t)
	}
	return tags, rows.Err()
}

func (r *tagRepo) AddGroupTag(ctx context.Context, groupID, tagID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO group_tags(group_id, tag_id) VALUES(?, ?) ON CONFLICT DO NOTHING`,
		groupID, tagID)
	if err != nil {
		return fmt.Errorf("add group tag: %w", err)
	}
	return nil
}

func (r *tagRepo) RemoveGroupTag(ctx context.Context, groupID, tagID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM group_tags WHERE group_id = ? AND tag_id = ?`,
		groupID, tagID)
	if err != nil {
		return fmt.Errorf("remove group tag: %w", err)
	}
	return nil
}

func (r *tagRepo) Save(ctx context.Context, t *domain.Tag) error {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.Scope == "" {
		t.Scope = domain.TagScopeUser
	}
	if t.Key == "" {
		t.Key = domain.TagKeyGeneral
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tags(id, key, value, scope) VALUES(?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET key = excluded.key, value = excluded.value, scope = excluded.scope`,
		t.ID, string(t.Key), t.Value, string(t.Scope))
	if err != nil {
		return fmt.Errorf("save tag: %w", err)
	}
	return nil
}

func (r *tagRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete tag %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DELETE FROM item_tags  WHERE tag_id = ?`,
		`DELETE FROM entry_tags WHERE tag_id = ?`,
		`DELETE FROM group_tags WHERE tag_id = ?`,
		`DELETE FROM tags       WHERE id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, id); err != nil {
			return fmt.Errorf("delete tag %s: %w", id, err)
		}
	}
	return tx.Commit()
}

func (r *tagRepo) DeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	var key, value string
	if err := r.db.QueryRowContext(ctx,
		`SELECT key, value FROM tags WHERE id = ?`, id,
	).Scan(&key, &value); err != nil {
		return nil, fmt.Errorf("deletion impact for tag %s: %w", id, err)
	}

	var itemCount, entryCount, groupCount int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_tags WHERE tag_id = ?`, id).Scan(&itemCount)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entry_tags WHERE tag_id = ?`, id).Scan(&entryCount)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_tags WHERE tag_id = ?`, id).Scan(&groupCount)

	impact := &domain.DeletionImpact{Mode: domain.DeletionModeUnlink, Impacts: []domain.DeletionImpactRow{}}
	if itemCount > 0 {
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind: "item", Count: itemCount, Label: "Items",
		})
	}
	if entryCount > 0 {
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind: "library_entry", Count: entryCount, Label: "Library Entries",
		})
	}
	if groupCount > 0 {
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind: "group", Count: groupCount, Label: "Groups",
		})
	}

	total := itemCount + entryCount + groupCount
	label := fmt.Sprintf("%q", value)
	if total == 0 {
		impact.Summary = fmt.Sprintf("Tag %s will be permanently deleted.", label)
	} else {
		impact.Summary = fmt.Sprintf("Tag %s will be removed from %d place(s) in the library.", label, total)
	}
	return impact, nil
}

var _ ports.TagRepository = (*tagRepo)(nil)
