package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
)

type itemRepo struct {
	db *sql.DB
}

// NewItemRepo returns an ItemRepository backed by SQLite.
func NewItemRepo(db *sql.DB) ports.ItemRepository {
	return &itemRepo{db: db}
}

const itemSelectCols = `
	id, content_type, library_entry_id, COALESCE(group_id, ''),
	title, overview, COALESCE(date, ''), sequence, runtime_seconds,
	monitored, status, cover_path, metadata, locked_fields, added_at, updated_at
`

func scanItem(row interface{ Scan(...any) error }) (*domain.Item, error) {
	var (
		i                        domain.Item
		contentType              string
		monitored                int
		status, metadata         string
		lockedFields             string
		date, addedAt, updatedAt string
	)
	if err := row.Scan(
		&i.ID, &contentType, &i.LibraryEntryID, &i.GroupID,
		&i.Title, &i.Overview, &date, &i.Sequence, &i.RuntimeSeconds,
		&monitored, &status, &i.CoverPath, &metadata, &lockedFields, &addedAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	i.ContentType = domain.ContentType(contentType)
	i.Monitored = intToBool(monitored)
	i.Status = domain.ItemStatus(status)
	i.Metadata = unmarshalMeta(metadata)
	i.LockedFields = unmarshalLockedFields(lockedFields)
	i.Date = strToDate(date)
	i.AddedAt = strToTime(addedAt)
	i.UpdatedAt = strToTime(updatedAt)
	return &i, nil
}

func (r *itemRepo) Get(ctx context.Context, id string) (*domain.Item, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+itemSelectCols+`FROM items WHERE id = ?`, id)
	item, err := scanItem(row)
	if err != nil {
		return nil, fmt.Errorf("get item %s: %w", id, err)
	}

	people, err := loadItemPeople(ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("load people for item %s: %w", id, err)
	}
	item.People = people

	tags, err := loadItemTags(ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("load tags for item %s: %w", id, err)
	}
	item.Tags = tags

	ids, err := loadExternalIDs(ctx, r.db, "item", id)
	if err != nil {
		return nil, fmt.Errorf("load external ids for item %s: %w", id, err)
	}
	item.ExternalIDs = ids

	mf, err := loadMediaFileByItemID(ctx, r.db, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load media file for item %s: %w", id, err)
	}
	item.MediaFile = mf

	return item, nil
}

func addItemContentTypes(w *whereClause, cts []domain.ContentType) {
	switch len(cts) {
	case 0:
		// no filter
	case 1:
		w.add("content_type = ?", string(cts[0]))
	default:
		ph := listPlaceholders(len(cts))
		args := make([]any, len(cts))
		for i, ct := range cts {
			args[i] = string(ct)
		}
		w.add("content_type IN ("+ph+")", args...) //nolint:gosec // ph is "?" markers built from len, not user input
	}
}

func buildItemWhere(f ports.ItemFilter) *whereClause {
	w := &whereClause{}
	if f.LibraryEntryID != "" {
		w.add("library_entry_id = ?", f.LibraryEntryID)
	}
	if f.GroupID != "" {
		w.add("group_id = ?", f.GroupID)
	}
	addItemContentTypes(w, f.ContentTypes)
	if f.Status != "" {
		w.add("status = ?", string(f.Status))
	}
	if f.Monitored != nil {
		w.add("monitored = ?", boolToInt(*f.Monitored))
	}
	if f.PersonID != "" {
		w.add("id IN (SELECT item_id FROM item_people WHERE person_id = ?)", f.PersonID)
	}
	if len(f.TagIDs) > 0 {
		ph := listPlaceholders(len(f.TagIDs))
		tagArgs := make([]any, len(f.TagIDs), len(f.TagIDs)+1)
		for i, id := range f.TagIDs {
			tagArgs[i] = id
		}
		w.add(
			fmt.Sprintf("id IN (SELECT item_id FROM item_tags WHERE tag_id IN (%s) GROUP BY item_id HAVING COUNT(DISTINCT tag_id) = ?)", ph),
			append(tagArgs, len(f.TagIDs))...,
		)
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
		sub := "SELECT it.item_id FROM item_tags it JOIN tags t ON t.id = it.tag_id WHERE " + strings.Join(conds, " AND ")
		w.add("id IN ("+sub+")", tagArgs...) //nolint:gosec // sub contains only hardcoded column predicates with ? parameters
	}
	if f.Search != "" {
		w.add("title LIKE ?", "%"+f.Search+"%")
	}
	return w
}

func itemOrderBy(f ports.ItemFilter) string {
	col := "date"
	switch f.Sort {
	case "title":
		col = "title"
	}

	dir := "DESC"
	switch strings.ToUpper(f.SortDir) {
	case "ASC":
		dir = "ASC"
	}

	if col == "date" {
		return col + " " + dir + ", sequence, title"
	}
	return col + " " + dir
}

func (r *itemRepo) List(ctx context.Context, f ports.ItemFilter) ([]*domain.Item, int, error) {
	w := buildItemWhere(f)

	where, args := w.build()

	var total int
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM items WHERE `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count items: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	queryArgs := append(args, limit, f.Offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+itemSelectCols+`FROM items WHERE `+where+
			` ORDER BY `+itemOrderBy(f)+` LIMIT ? OFFSET ?`,
		queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// rows is now exhausted and the connection is released back to the pool.
	// Batch-load secondary data — no nested queries while a cursor is open.
	if err := attachItemPeopleBatch(ctx, r.db, items); err != nil {
		return nil, 0, fmt.Errorf("load people for items: %w", err)
	}
	if err := attachExternalIDsBatch(ctx, r.db, "item", items,
		func(item *domain.Item) string { return item.ID },
		func(item *domain.Item, ids []domain.ExternalID) { item.ExternalIDs = ids },
	); err != nil {
		return nil, 0, fmt.Errorf("load external ids for items: %w", err)
	}

	return items, total, nil
}

func (r *itemRepo) Save(ctx context.Context, item *domain.Item) error {
	if item.ID == "" {
		item.ID = newID()
	}
	now := nowStr()
	if item.AddedAt.IsZero() {
		item.AddedAt = strToTime(now)
	}
	item.UpdatedAt = strToTime(now)

	var groupID *string
	if item.GroupID != "" {
		groupID = &item.GroupID
	}

	var date *string
	if !item.Date.IsZero() {
		s := dateToStr(item.Date)
		date = &s
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save item: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx, `
		INSERT INTO items(
			id, content_type, library_entry_id, group_id,
			title, overview, date, sequence, runtime_seconds,
			monitored, status, cover_path, metadata, locked_fields, added_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			content_type     = excluded.content_type,
			library_entry_id = excluded.library_entry_id,
			group_id         = excluded.group_id,
			title            = excluded.title,
			overview         = excluded.overview,
			date             = excluded.date,
			sequence         = excluded.sequence,
			runtime_seconds  = excluded.runtime_seconds,
			monitored        = excluded.monitored,
			status           = excluded.status,
			cover_path       = excluded.cover_path,
			metadata         = excluded.metadata,
			locked_fields    = excluded.locked_fields,
			updated_at       = excluded.updated_at`,
		item.ID, string(item.ContentType), item.LibraryEntryID, groupID,
		item.Title, item.Overview, date, item.Sequence, item.RuntimeSeconds,
		boolToInt(item.Monitored), string(item.Status), item.CoverPath,
		marshalMeta(item.Metadata), marshalLockedFields(item.LockedFields), timeToStr(item.AddedAt), now,
	); err != nil {
		return fmt.Errorf("upsert item: %w", err)
	}

	if err := saveItemPeople(ctx, tx, item.ID, item.People); err != nil {
		return fmt.Errorf("save people: %w", err)
	}
	if err := saveItemTags(ctx, tx, item.ID, item.Tags); err != nil {
		return fmt.Errorf("save tags: %w", err)
	}
	if err := saveExternalIDs(ctx, tx, "item", item.ID, item.ExternalIDs); err != nil {
		return fmt.Errorf("save external ids: %w", err)
	}

	return tx.Commit()
}

func (r *itemRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete item %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DELETE FROM item_people  WHERE item_id = ?`,
		`DELETE FROM item_tags    WHERE item_id = ?`,
		`DELETE FROM downloads    WHERE item_id = ?`,
		`DELETE FROM media_files  WHERE item_id = ?`,
		`UPDATE releases SET item_id = NULL WHERE item_id = ?`,
		`DELETE FROM external_ids WHERE entity_type = 'item' AND entity_id = ?`,
		`DELETE FROM items        WHERE id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, id); err != nil {
			return fmt.Errorf("delete item %s: %w", id, err)
		}
	}
	return tx.Commit()
}

func (r *itemRepo) DeleteByGroup(ctx context.Context, groupID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete items for group %s: %w", groupID, err)
	}
	defer func() { _ = tx.Rollback() }()

	sub := `(SELECT id FROM items WHERE group_id = ?)`
	stmts := []string{
		`DELETE FROM item_people  WHERE item_id IN ` + sub,
		`DELETE FROM item_tags    WHERE item_id IN ` + sub,
		`DELETE FROM downloads    WHERE item_id IN ` + sub,
		`DELETE FROM media_files  WHERE item_id IN ` + sub,
		`UPDATE releases SET item_id = NULL WHERE item_id IN ` + sub,
		`DELETE FROM external_ids WHERE entity_type = 'item' AND entity_id IN ` + sub,
		`DELETE FROM items        WHERE group_id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, groupID); err != nil {
			return fmt.Errorf("delete items for group %s: %w", groupID, err)
		}
	}
	return tx.Commit()
}

func (r *itemRepo) DeleteByLibraryEntry(ctx context.Context, entryID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete items for entry %s: %w", entryID, err)
	}
	defer func() { _ = tx.Rollback() }()

	sub := `(SELECT id FROM items WHERE library_entry_id = ?)`
	stmts := []string{
		`DELETE FROM item_people  WHERE item_id IN ` + sub,
		`DELETE FROM item_tags    WHERE item_id IN ` + sub,
		`DELETE FROM downloads    WHERE item_id IN ` + sub,
		`DELETE FROM media_files  WHERE item_id IN ` + sub,
		`UPDATE releases SET item_id = NULL WHERE item_id IN ` + sub,
		`DELETE FROM external_ids WHERE entity_type = 'item' AND entity_id IN ` + sub,
		`DELETE FROM items        WHERE library_entry_id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, entryID); err != nil {
			return fmt.Errorf("delete items for entry %s: %w", entryID, err)
		}
	}
	return tx.Commit()
}

func (r *itemRepo) DeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	var title string
	if err := r.db.QueryRowContext(ctx,
		`SELECT title FROM items WHERE id = ?`, id,
	).Scan(&title); err != nil {
		return nil, fmt.Errorf("deletion impact for item %s: %w", id, err)
	}

	var fileCount int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_files WHERE item_id = ?`, id).Scan(&fileCount)

	impact := &domain.DeletionImpact{Mode: domain.DeletionModeDestroy, Impacts: []domain.DeletionImpactRow{}}
	if fileCount > 0 {
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind: "media_file", Count: fileCount, Label: "Files",
		})
		impact.Summary = fmt.Sprintf("Deleting %s will permanently remove it and its associated file.", title)
	} else {
		impact.Summary = fmt.Sprintf("Deleting %s will permanently remove it from the library.", title)
	}
	return impact, nil
}

var _ ports.ItemRepository = (*itemRepo)(nil)

// loadMediaFileByItemID returns the MediaFile for an item.
// Returns nil and sql.ErrNoRows when no media file exists for the item.
func loadMediaFileByItemID(ctx context.Context, db *sql.DB, itemID string) (*domain.MediaFile, error) {
	row := db.QueryRowContext(ctx,
		`SELECT`+mediaFileSelectCols+`FROM media_files WHERE item_id = ?`, itemID)
	return scanMediaFile(row)
}
