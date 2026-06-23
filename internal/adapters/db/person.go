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

	ids, err := loadExternalIDs(ctx, r.db, "person", id)
	if err != nil {
		return nil, fmt.Errorf("load external ids for person %s: %w", id, err)
	}
	p.ExternalIDs = ids

	return p, nil
}

func (r *personRepo) List(ctx context.Context, f ports.PersonFilter) ([]*domain.Person, int, error) {
	w := &whereClause{}

	if f.Monitored != nil {
		w.add("monitored = ?", boolToInt(*f.Monitored))
	}
	if f.Role != "" {
		w.add(`id IN (
			SELECT person_id FROM item_people WHERE role = ?
			UNION
			SELECT person_id FROM entry_people WHERE role = ?
		)`, string(f.Role), string(f.Role))
	}
	if f.ContentType != "" {
		w.add(`id IN (
			SELECT ip.person_id FROM item_people ip
			JOIN items i ON i.id = ip.item_id
			WHERE i.content_type = ?
			UNION
			SELECT ep.person_id FROM entry_people ep
			JOIN library_entries le ON le.id = ep.library_entry_id
			WHERE le.content_type = ?
		)`, string(f.ContentType), string(f.ContentType))
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
		// Load aliases in list view (needed for search result display).
		p.Aliases, err = loadAliases(ctx, r.db, p.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("load aliases for %s: %w", p.ID, err)
		}
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if err := attachExternalIDsBatch(ctx, r.db, "person", people,
		func(p *domain.Person) string { return p.ID },
		func(p *domain.Person, ids []domain.ExternalID) { p.ExternalIDs = ids },
	); err != nil {
		return nil, 0, fmt.Errorf("load external ids for people: %w", err)
	}

	return people, total, nil
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
	if err := saveExternalIDs(ctx, tx, "person", p.ID, p.ExternalIDs); err != nil {
		return fmt.Errorf("save external ids: %w", err)
	}

	return tx.Commit()
}

func (r *personRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM people WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete person %s: %w", id, err)
	}
	return nil
}

var _ ports.PersonRepository = (*personRepo)(nil)
