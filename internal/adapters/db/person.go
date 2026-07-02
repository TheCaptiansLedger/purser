package db

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
)

type personRepo struct {
	db *sql.DB
}

// NewPersonRepo returns a PersonRepository backed by SQLite.
func NewPersonRepo(db *sql.DB) ports.PersonRepository {
	return &personRepo{db: db}
}

const personSelectCols = `
	id, name, sort_name, overview, monitored, monitor_mode, image_path, metadata, locked_fields, added_at
`

func scanPerson(row interface{ Scan(...any) error }) (*domain.Person, error) {
	var (
		p            domain.Person
		monitorMode  string
		monitored    int
		metadata     string
		lockedFields string
		addedAt      string
	)
	if err := row.Scan(
		&p.ID, &p.Name, &p.SortName, &p.Overview,
		&monitored, &monitorMode, &p.ImagePath, &metadata, &lockedFields, &addedAt,
	); err != nil {
		return nil, err
	}
	p.Monitored = intToBool(monitored)
	p.MonitorMode = domain.MonitorMode(monitorMode)
	p.Metadata = unmarshalMeta(metadata)
	p.LockedFields = unmarshalLockedFields(lockedFields)
	p.AddedAt = strToTime(addedAt)
	return &p, nil
}

func (r *personRepo) Get(ctx context.Context, id string) (*domain.Person, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+personSelectCols+`FROM people WHERE id = ?`, id)
	p, err := scanPerson(row)
	if err != nil {
		return nil, fmt.Errorf("get person %s: %w", id, err)
	}

	aliases, err := loadAliases(ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("load aliases for person %s: %w", id, err)
	}
	p.Aliases = aliases

	roles, err := loadPersonRoles(ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("load roles for person %s: %w", id, err)
	}
	p.Roles = roles

	ids, err := loadExternalIDs(ctx, r.db, "person", id)
	if err != nil {
		return nil, fmt.Errorf("load external ids for person %s: %w", id, err)
	}
	p.ExternalIDs = ids

	return p, nil
}

func addPersonContentTypes(w *whereClause, cts []domain.ContentType) {
	switch len(cts) {
	case 0:
		// no filter
	case 1:
		ct := string(cts[0])
		w.add(`id IN (
			SELECT ip.person_id FROM item_people ip
			JOIN items i ON i.id = ip.item_id
			WHERE i.content_type = ?
			UNION
			SELECT ep.person_id FROM entry_people ep
			JOIN library_entries le ON le.id = ep.library_entry_id
			WHERE le.content_type = ?
		)`, ct, ct)
	default:
		ph := listPlaceholders(len(cts))
		args := make([]any, 0, len(cts)*2)
		for _, ct := range cts {
			args = append(args, string(ct))
		}
		for _, ct := range cts {
			args = append(args, string(ct))
		}
		w.add(`id IN (
			SELECT ip.person_id FROM item_people ip
			JOIN items i ON i.id = ip.item_id
			WHERE i.content_type IN (`+ph+`)
			UNION
			SELECT ep.person_id FROM entry_people ep
			JOIN library_entries le ON le.id = ep.library_entry_id
			WHERE le.content_type IN (`+ph+`)
		)`, args...) //nolint:gosec // ph is "?" markers built from len, not user input
	}
}

func (r *personRepo) List(ctx context.Context, f ports.PersonFilter) ([]*domain.Person, int, error) {
	w := &whereClause{}

	if f.Monitored != nil {
		w.add("monitored = ?", boolToInt(*f.Monitored))
	}
	if f.Role != "" {
		w.add(`id IN (SELECT person_id FROM person_roles WHERE role = ?)`, string(f.Role))
	}
	addPersonContentTypes(w, f.ContentTypes)
	if f.Unlinked {
		w.add(`NOT EXISTS (SELECT 1 FROM item_people ip WHERE ip.person_id = people.id)` +
			` AND NOT EXISTS (SELECT 1 FROM entry_people ep WHERE ep.person_id = people.id)`)
	}
	if f.Search != "" {
		w.add(`(name LIKE ? OR id IN (
			SELECT person_id FROM people_aliases WHERE alias LIKE ?
		))`, "%"+f.Search+"%", "%"+f.Search+"%")
	}

	where, args := w.build()

	var total int
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM people WHERE `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count people: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	queryArgs := append(args, limit, f.Offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+personSelectCols+`FROM people WHERE `+where+
			` ORDER BY sort_name, name LIMIT ? OFFSET ?`,
		queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list people: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var people []*domain.Person
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, 0, err
		}
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// rows is now exhausted and the connection is released back to the pool.
	// Batch-load secondary data — no nested queries while a cursor is open.
	if err := attachAliasesBatch(ctx, r.db, people); err != nil {
		return nil, 0, fmt.Errorf("load aliases for people: %w", err)
	}
	if err := attachPersonRolesBatch(ctx, r.db, people); err != nil {
		return nil, 0, fmt.Errorf("load roles for people: %w", err)
	}
	if err := attachExternalIDsBatch(ctx, r.db, "person", people,
		func(p *domain.Person) string { return p.ID },
		func(p *domain.Person, ids []domain.ExternalID) { p.ExternalIDs = ids },
	); err != nil {
		return nil, 0, fmt.Errorf("load external ids for people: %w", err)
	}

	return people, total, nil
}

func (r *personRepo) ListRoles(ctx context.Context) ([]domain.PersonRoleCount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT role, COUNT(DISTINCT person_id) FROM person_roles GROUP BY role ORDER BY role`)
	if err != nil {
		return nil, fmt.Errorf("list person roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.PersonRoleCount
	for rows.Next() {
		var rc domain.PersonRoleCount
		var role string
		if err := rows.Scan(&role, &rc.Count); err != nil {
			return nil, fmt.Errorf("scan role count: %w", err)
		}
		rc.Role = domain.PersonRole(role)
		out = append(out, rc)
	}
	return out, rows.Err()
}

func (r *personRepo) Save(ctx context.Context, p *domain.Person) error {
	if p.ID == "" {
		p.ID = newID()
	}
	if p.AddedAt.IsZero() {
		p.AddedAt = strToTime(nowStr())
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save person: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx, `
		INSERT INTO people(id, name, sort_name, overview, monitored, monitor_mode, image_path, metadata, locked_fields, added_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name          = excluded.name,
			sort_name     = excluded.sort_name,
			overview      = excluded.overview,
			monitored     = excluded.monitored,
			monitor_mode  = excluded.monitor_mode,
			image_path    = excluded.image_path,
			metadata      = excluded.metadata,
			locked_fields = excluded.locked_fields`,
		p.ID, p.Name, p.SortName, p.Overview,
		boolToInt(p.Monitored), string(p.MonitorMode), p.ImagePath,
		marshalMeta(p.Metadata), marshalLockedFields(p.LockedFields), timeToStr(p.AddedAt),
	); err != nil {
		return fmt.Errorf("upsert person: %w", err)
	}

	if err := saveAliases(ctx, tx, p.ID, p.Aliases); err != nil {
		return fmt.Errorf("save aliases: %w", err)
	}
	if err := savePersonRoles(ctx, tx, p.ID, p.Roles); err != nil {
		return fmt.Errorf("save roles: %w", err)
	}
	if err := saveExternalIDs(ctx, tx, "person", p.ID, p.ExternalIDs); err != nil {
		return fmt.Errorf("save external ids: %w", err)
	}

	return tx.Commit()
}

func (r *personRepo) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete person %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DELETE FROM item_people   WHERE person_id = ?`,
		`DELETE FROM entry_people  WHERE person_id = ?`,
		`DELETE FROM people_aliases WHERE person_id = ?`,
		`DELETE FROM person_roles  WHERE person_id = ?`,
		`DELETE FROM external_ids  WHERE entity_type = 'person' AND entity_id = ?`,
		`DELETE FROM people        WHERE id = ?`,
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s, id); err != nil {
			return fmt.Errorf("delete person %s: %w", id, err)
		}
	}
	return tx.Commit()
}

func (r *personRepo) DeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	var name string
	if err := r.db.QueryRowContext(ctx,
		`SELECT name FROM people WHERE id = ?`, id,
	).Scan(&name); err != nil {
		return nil, fmt.Errorf("deletion impact for person %s: %w", id, err)
	}

	impact := &domain.DeletionImpact{Mode: domain.DeletionModeUnlink, Impacts: []domain.DeletionImpactRow{}}
	total := 0

	// Item credits — group by content type so "Scenes: 3, Tracks: 1" is possible.
	itemRows, err := r.db.QueryContext(ctx, `
		SELECT le.content_type, COUNT(*)
		FROM item_people ip
		JOIN items i ON i.id = ip.item_id
		JOIN library_entries le ON le.id = i.library_entry_id
		WHERE ip.person_id = ?
		GROUP BY le.content_type`, id)
	if err != nil {
		return nil, fmt.Errorf("deletion impact items for person %s: %w", id, err)
	}
	defer func() { _ = itemRows.Close() }()
	for itemRows.Next() {
		var ct string
		var count int
		if err := itemRows.Scan(&ct, &count); err != nil {
			return nil, err
		}
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind:  "item_" + ct,
			Count: count,
			Label: domain.ContentType(ct).ItemLabel(),
		})
		total += count
	}

	// Entry credits — group by kind so "Artists: 1, Studios: 2" is possible.
	entryRows, err := r.db.QueryContext(ctx, `
		SELECT le.kind, COUNT(*)
		FROM entry_people ep
		JOIN library_entries le ON le.id = ep.library_entry_id
		WHERE ep.person_id = ?
		GROUP BY le.kind`, id)
	if err != nil {
		return nil, fmt.Errorf("deletion impact entries for person %s: %w", id, err)
	}
	defer func() { _ = entryRows.Close() }()
	for entryRows.Next() {
		var kind string
		var count int
		if err := entryRows.Scan(&kind, &count); err != nil {
			return nil, err
		}
		impact.Impacts = append(impact.Impacts, domain.DeletionImpactRow{
			Kind:  "entry_" + kind,
			Count: count,
			Label: domain.Kind(kind).EntryLabel(),
		})
		total += count
	}

	if total == 0 {
		impact.Summary = fmt.Sprintf("%s will be permanently deleted.", name)
	} else {
		impact.Summary = fmt.Sprintf("%s will be removed from %d credit(s) across the library.", name, total)
	}
	return impact, nil
}

var _ ports.PersonRepository = (*personRepo)(nil)
