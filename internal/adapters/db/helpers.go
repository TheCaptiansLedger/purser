package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"purser/internal/domain"
	"strings"
	"time"

	"github.com/google/uuid"
)

func newID() string {
	return uuid.New().String()
}

func nowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func timeToStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func strToTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func dateToStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func strToDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func marshalMeta(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func unmarshalMeta(s string) map[string]any {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

func marshalLockedFields(fields []string) string {
	if len(fields) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

func unmarshalLockedFields(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var fields []string
	_ = json.Unmarshal([]byte(s), &fields)
	return fields
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}

// whereClause builds a dynamic WHERE clause from filter conditions.
type whereClause struct {
	parts []string
	args  []any
}

func (w *whereClause) add(expr string, args ...any) {
	w.parts = append(w.parts, expr)
	w.args = append(w.args, args...)
}

func (w *whereClause) build() (string, []any) {
	if len(w.parts) == 0 {
		return "1=1", nil
	}
	return strings.Join(w.parts, " AND "), w.args
}

// ── External IDs ──────────────────────────────────────────────────────────────

func loadExternalIDs(ctx context.Context, db *sql.DB, entityType, entityID string) ([]domain.ExternalID, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT source, value FROM external_ids
		 WHERE entity_type = ? AND entity_id = ?
		 ORDER BY source`,
		entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []domain.ExternalID
	for rows.Next() {
		var src, val string
		if err := rows.Scan(&src, &val); err != nil {
			return nil, err
		}
		ids = append(ids, domain.ExternalID{Source: domain.ExternalIDSource(src), Value: val})
	}
	return ids, rows.Err()
}

// loadExternalIDsBatch fetches external IDs for a set of entity IDs in a
// single query and returns a map of entityID → []ExternalID. Entities with
// no external IDs are absent from the map (not present with a nil/empty slice).
func loadExternalIDsBatch(ctx context.Context, db *sql.DB, entityType string, entityIDs []string) (map[string][]domain.ExternalID, error) {
	if len(entityIDs) == 0 {
		return map[string][]domain.ExternalID{}, nil
	}
	placeholders := strings.Repeat("?,", len(entityIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, 1+len(entityIDs))
	args = append(args, entityType)
	for _, id := range entityIDs {
		args = append(args, id)
	}
	q := `SELECT entity_id, source, value FROM external_ids WHERE entity_type = ? AND entity_id IN (` + placeholders + `) ORDER BY entity_id, source` //nolint:gosec // placeholders are "?" parameter markers built from len(entityIDs), not user input
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string][]domain.ExternalID)
	for rows.Next() {
		var entityID, src, val string
		if err := rows.Scan(&entityID, &src, &val); err != nil {
			return nil, err
		}
		out[entityID] = append(out[entityID], domain.ExternalID{Source: domain.ExternalIDSource(src), Value: val})
	}
	return out, rows.Err()
}

// attachExternalIDsBatch loads external IDs for a batch of entities in one
// query and assigns them in place. getID extracts each entity's ID; setIDs
// stores the loaded slice back onto the entity.
func attachExternalIDsBatch[T any](
	ctx context.Context, db *sql.DB, entityType string, items []*T,
	getID func(*T) string, setIDs func(*T, []domain.ExternalID),
) error {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = getID(item)
	}
	extIDs, err := loadExternalIDsBatch(ctx, db, entityType, ids)
	if err != nil {
		return err
	}
	for _, item := range items {
		setIDs(item, extIDs[getID(item)])
	}
	return nil
}

func saveExternalIDs(ctx context.Context, tx *sql.Tx, entityType, entityID string, ids []domain.ExternalID) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM external_ids WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID); err != nil {
		return err
	}
	for _, id := range ids {
		// Remove any orphaned row with the same (entity_type, source, value) pointing
		// to a different entity — happens when an entity is deleted without cascading
		// the external_ids table (no FK exists there).
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM external_ids WHERE entity_type = ? AND source = ? AND value = ? AND entity_id != ?`,
			entityType, string(id.Source), id.Value, entityID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO external_ids(entity_type, entity_id, source, value) VALUES(?, ?, ?, ?)`,
			entityType, entityID, string(id.Source), id.Value); err != nil {
			return err
		}
	}
	return nil
}

// ── Tags ──────────────────────────────────────────────────────────────────────

func loadItemTags(ctx context.Context, db *sql.DB, itemID string) ([]domain.Tag, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.id, t.key, t.value, t.scope FROM tags t
		 JOIN item_tags it ON it.tag_id = t.id
		 WHERE it.item_id = ?
		 ORDER BY t.value`,
		itemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanTags(rows)
}

func loadEntryTags(ctx context.Context, db *sql.DB, entryID string) ([]domain.Tag, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.id, t.key, t.value, t.scope FROM tags t
		 JOIN entry_tags et ON et.tag_id = t.id
		 WHERE et.library_entry_id = ?
		 ORDER BY t.value`,
		entryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanTags(rows)
}

func scanTags(rows *sql.Rows) ([]domain.Tag, error) {
	var tags []domain.Tag
	for rows.Next() {
		var t domain.Tag
		var key, scope string
		if err := rows.Scan(&t.ID, &key, &t.Value, &scope); err != nil {
			return nil, err
		}
		t.Key = domain.TagKey(key)
		t.Scope = domain.TagScope(scope)
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func saveItemTags(ctx context.Context, tx *sql.Tx, itemID string, tags []domain.Tag) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_tags WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO item_tags(item_id, tag_id) VALUES(?, ?) ON CONFLICT DO NOTHING`,
			itemID, t.ID); err != nil {
			return err
		}
	}
	return nil
}

func loadGroupTags(ctx context.Context, db *sql.DB, groupID string) ([]domain.Tag, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.id, t.key, t.value, t.scope FROM tags t
		 JOIN group_tags gt ON gt.tag_id = t.id
		 WHERE gt.group_id = ?
		 ORDER BY t.value`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanTags(rows)
}

func saveEntryTags(ctx context.Context, tx *sql.Tx, entryID string, tags []domain.Tag) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM entry_tags WHERE library_entry_id = ?`, entryID); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO entry_tags(library_entry_id, tag_id) VALUES(?, ?) ON CONFLICT DO NOTHING`,
			entryID, t.ID); err != nil {
			return err
		}
	}
	return nil
}

// ── Item people ───────────────────────────────────────────────────────────────

func loadItemPeople(ctx context.Context, db *sql.DB, itemID string) ([]domain.ItemPerson, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ip.person_id, ip.role, p.name, p.sort_name, p.image_path
		 FROM item_people ip
		 JOIN people p ON p.id = ip.person_id
		 WHERE ip.item_id = ?
		 ORDER BY p.sort_name, p.name`,
		itemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var people []domain.ItemPerson
	for rows.Next() {
		var (
			ip   domain.ItemPerson
			role string
			p    domain.Person
		)
		if err := rows.Scan(&ip.PersonID, &role, &p.Name, &p.SortName, &p.ImagePath); err != nil {
			return nil, err
		}
		ip.Role = domain.PersonRole(role)
		p.ID = ip.PersonID
		ip.Person = &p
		people = append(people, ip)
	}
	return people, rows.Err()
}

func saveItemPeople(ctx context.Context, tx *sql.Tx, itemID string, people []domain.ItemPerson) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_people WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for _, ip := range people {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO item_people(item_id, person_id, role) VALUES(?, ?, ?)
			 ON CONFLICT DO NOTHING`,
			itemID, ip.PersonID, string(ip.Role)); err != nil {
			return err
		}
	}
	return nil
}

// ── Entry people (artist members) ────────────────────────────────────────────

func loadEntryPeople(ctx context.Context, db *sql.DB, entryID string) ([]domain.EntryPerson, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT ep.person_id, ep.role, ep.start_date, ep.end_date,
		        p.name, p.sort_name, p.image_path
		 FROM entry_people ep
		 JOIN people p ON p.id = ep.person_id
		 WHERE ep.library_entry_id = ?
		 ORDER BY p.sort_name, p.name`,
		entryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []domain.EntryPerson
	for rows.Next() {
		var (
			ep                 domain.EntryPerson
			startDate, endDate string
			p                  domain.Person
		)
		if err := rows.Scan(
			&ep.PersonID, &ep.Role, &startDate, &endDate,
			&p.Name, &p.SortName, &p.ImagePath,
		); err != nil {
			return nil, err
		}
		ep.StartDate = strToDate(startDate)
		ep.EndDate = strToDate(endDate)
		p.ID = ep.PersonID
		ep.Person = &p
		members = append(members, ep)
	}
	return members, rows.Err()
}

// ── Batch loaders (used by List methods to avoid nested queries) ──────────────

// attachAliasesBatch loads aliases for a slice of people in one query.
func attachAliasesBatch(ctx context.Context, db *sql.DB, people []*domain.Person) error {
	if len(people) == 0 {
		return nil
	}
	ids := make([]any, len(people))
	for i, p := range people {
		ids[i] = p.ID
	}
	rows, err := db.QueryContext(ctx,
		`SELECT person_id, alias FROM people_aliases WHERE person_id IN (`+listPlaceholders(len(ids))+`) ORDER BY person_id, alias`, //nolint:gosec // listPlaceholders returns only "?" markers, not user input
		ids...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	m := make(map[string][]string)
	for rows.Next() {
		var pid, alias string
		if err := rows.Scan(&pid, &alias); err != nil {
			return err
		}
		m[pid] = append(m[pid], alias)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range people {
		p.Aliases = m[p.ID]
	}
	return nil
}

// attachPersonRolesBatch loads roles for a slice of people in one query.
func attachPersonRolesBatch(ctx context.Context, db *sql.DB, people []*domain.Person) error {
	if len(people) == 0 {
		return nil
	}
	ids := make([]any, len(people))
	for i, p := range people {
		ids[i] = p.ID
	}
	rows, err := db.QueryContext(ctx,
		`SELECT person_id, role FROM person_roles WHERE person_id IN (`+listPlaceholders(len(ids))+`) ORDER BY person_id, role`, //nolint:gosec // listPlaceholders returns only "?" markers, not user input
		ids...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	m := make(map[string][]domain.PersonRole)
	for rows.Next() {
		var pid, role string
		if err := rows.Scan(&pid, &role); err != nil {
			return err
		}
		m[pid] = append(m[pid], domain.PersonRole(role))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range people {
		p.Roles = m[p.ID]
	}
	return nil
}

// attachItemPeopleBatch loads item_people for a slice of items in one query.
func attachItemPeopleBatch(ctx context.Context, db *sql.DB, items []*domain.Item) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]any, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	q := `SELECT ip.item_id, ip.person_id, ip.role, p.name, p.sort_name, p.image_path FROM item_people ip JOIN people p ON p.id = ip.person_id WHERE ip.item_id IN (` + listPlaceholders(len(ids)) + `) ORDER BY ip.item_id, p.sort_name, p.name` //nolint:gosec // listPlaceholders returns only "?" markers, not user input
	rows, err := db.QueryContext(ctx, q, ids...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	m := make(map[string][]domain.ItemPerson)
	for rows.Next() {
		var (
			itemID string
			ip     domain.ItemPerson
			role   string
			p      domain.Person
		)
		if err := rows.Scan(&itemID, &ip.PersonID, &role, &p.Name, &p.SortName, &p.ImagePath); err != nil {
			return err
		}
		ip.Role = domain.PersonRole(role)
		p.ID = ip.PersonID
		ip.Person = &p
		m[itemID] = append(m[itemID], ip)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		item.People = m[item.ID]
	}
	return nil
}

// ── People aliases ────────────────────────────────────────────────────────────

func loadAliases(ctx context.Context, db *sql.DB, personID string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT alias FROM people_aliases WHERE person_id = ? ORDER BY alias`,
		personID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var aliases []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		aliases = append(aliases, a)
	}
	return aliases, rows.Err()
}

func saveAliases(ctx context.Context, tx *sql.Tx, personID string, aliases []string) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM people_aliases WHERE person_id = ?`, personID); err != nil {
		return err
	}
	for _, a := range aliases {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO people_aliases(person_id, alias) VALUES(?, ?) ON CONFLICT DO NOTHING`,
			personID, a); err != nil {
			return err
		}
	}
	return nil
}

// ── Person roles ──────────────────────────────────────────────────────────────

func loadPersonRoles(ctx context.Context, db *sql.DB, personID string) ([]domain.PersonRole, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT role FROM person_roles WHERE person_id = ? ORDER BY role`,
		personID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var roles []domain.PersonRole
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		roles = append(roles, domain.PersonRole(r))
	}
	return roles, rows.Err()
}

func savePersonRoles(ctx context.Context, tx *sql.Tx, personID string, roles []domain.PersonRole) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM person_roles WHERE person_id = ?`, personID); err != nil {
		return err
	}
	for _, r := range roles {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO person_roles(person_id, role) VALUES(?, ?) ON CONFLICT DO NOTHING`,
			personID, string(r)); err != nil {
			return err
		}
	}
	return nil
}
