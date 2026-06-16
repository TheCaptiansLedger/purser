package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"purser/internal/domain"
	"purser/internal/ports"
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
	var scope, category string
	if err := r.db.QueryRowContext(ctx,
		`SELECT id, name, scope, category FROM tags WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &scope, &category); err != nil {
		return nil, fmt.Errorf("get tag %s: %w", id, err)
	}
	t.Scope = domain.TagScope(scope)
	t.Category = domain.TagCategory(category)
	return &t, nil
}

func (r *tagRepo) List(ctx context.Context, f ports.TagFilter) ([]*domain.Tag, error) {
	q := `SELECT DISTINCT t.id, t.name, t.scope, t.category FROM tags t`
	var args []any
	var conditions []string

	if len(f.ContentTypes) > 0 {
		placeholders := make([]string, len(f.ContentTypes))
		for i, ct := range f.ContentTypes {
			placeholders[i] = "?"
			args = append(args, string(ct))
		}
		in := strings.Join(placeholders, ",")
		ctArgs := make([]any, len(f.ContentTypes))
		copy(ctArgs, args)
		conditions = append(conditions, fmt.Sprintf(`(
			EXISTS (SELECT 1 FROM item_tags it JOIN items i ON i.id = it.item_id WHERE it.tag_id = t.id AND i.content_type IN (%s))
			OR EXISTS (SELECT 1 FROM entry_tags et JOIN library_entries le ON le.id = et.library_entry_id WHERE et.tag_id = t.id AND le.content_type IN (%s))
		)`, in, in))
		// duplicate the args for the second IN clause
		args = append(args, ctArgs...)
	}

	if f.Scope != "" {
		conditions = append(conditions, `t.scope = ?`)
		args = append(args, string(f.Scope))
	}

	if f.Category != "" {
		conditions = append(conditions, `t.category = ?`)
		args = append(args, string(f.Category))
	}

	if len(conditions) > 0 {
		q += ` WHERE ` + strings.Join(conditions, ` AND `)
	}
	q += ` ORDER BY t.name`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []*domain.Tag
	for rows.Next() {
		var t domain.Tag
		var sc, cat string
		if err := rows.Scan(&t.ID, &t.Name, &sc, &cat); err != nil {
			return nil, err
		}
		t.Scope = domain.TagScope(sc)
		t.Category = domain.TagCategory(cat)
		tags = append(tags, &t)
	}
	return tags, rows.Err()
}

func (r *tagRepo) Save(ctx context.Context, t *domain.Tag) error {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.Scope == "" {
		t.Scope = domain.TagScopeUser
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tags(id, name, scope, category) VALUES(?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, scope = excluded.scope, category = excluded.category`,
		t.ID, t.Name, string(t.Scope), string(t.Category))
	if err != nil {
		return fmt.Errorf("save tag: %w", err)
	}
	return nil
}

func (r *tagRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete tag %s: %w", id, err)
	}
	return nil
}

var _ ports.TagRepository = (*tagRepo)(nil)
