// Package tagtest is the shared contract test suite for the
// ports.TagRepository port. See internal/ports/persontest for the
// convention this follows.
package tagtest

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// NewRepositoryFunc returns a fresh, empty TagRepository for the duration
// of a single subtest.
type NewRepositoryFunc func(t *testing.T) ports.TagRepository

// TestTagRepository runs the shared TagRepository contract against
// newRepo.
func TestTagRepository(t *testing.T, newRepo NewRepositoryFunc) {
	t.Helper()

	t.Run("get on empty repository returns ErrNotFound", func(t *testing.T) { testGetOnEmptyNotFound(t, newRepo) })
	t.Run("create then get round-trips the tag", func(t *testing.T) { testCreateThenGet(t, newRepo) })
	t.Run("create with the same ID and identity is idempotent", func(t *testing.T) { testCreateSameIDSameIdentity(t, newRepo) })
	t.Run("create with a different ID but the same identity returns the existing tag", func(t *testing.T) { testCreateGetOrCreateHit(t, newRepo) })
	t.Run("create with the same ID but a different identity returns ErrConflict", func(t *testing.T) { testCreateIDCollision(t, newRepo) })
	t.Run("create with the same key/value under a different scope creates a second tag", func(t *testing.T) { testCreateDifferentScopeIsDistinct(t, newRepo) })
	t.Run("update replaces an existing tag", func(t *testing.T) { testUpdate(t, newRepo) })
	t.Run("update on a missing tag returns ErrNotFound", func(t *testing.T) { testUpdateMissing(t, newRepo) })
	t.Run("update renaming into an identity owned by a different tag returns ErrConflict", func(t *testing.T) { testUpdateRenameConflict(t, newRepo) })
	t.Run("update renaming frees the old identity for reuse", func(t *testing.T) { testUpdateRenameFreesOldIdentity(t, newRepo) })
	t.Run("delete removes a tag", func(t *testing.T) { testDelete(t, newRepo) })
	t.Run("delete on a missing tag returns ErrNotFound", func(t *testing.T) { testDeleteMissing(t, newRepo) })
	t.Run("delete frees the identity for reuse by a new tag", func(t *testing.T) { testDeleteFreesIdentity(t, newRepo) })
	t.Run("list returns every created tag across pages", func(t *testing.T) { testListPaginates(t, newRepo) })
	t.Run("delete batch removes every tag atomically", func(t *testing.T) { testDeleteBatch(t, newRepo) })
	t.Run("delete batch with one missing id rolls back the whole batch", func(t *testing.T) { testDeleteBatchRollsBackOnMissing(t, newRepo) })
	t.Run("delete batch frees every identity for reuse", func(t *testing.T) { testDeleteBatchFreesIdentities(t, newRepo) })
}

func mustCreate(t *testing.T, r ports.TagRepository, tag *domain.Tag) {
	t.Helper()
	if err := r.Create(context.Background(), tag); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func testGetOnEmptyNotFound(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	_, err := r.Get(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty repository returned %v, want ErrNotFound", err)
	}
}

func testCreateThenGet(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	tag := sampleTag("t1")
	mustCreate(t, r, tag)

	got, err := r.Get(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != tag.Value {
		t.Fatalf("Get returned Value %q, want %q", got.Value, tag.Value)
	}
}

// testCreateSameIDSameIdentity covers re-issuing the exact same Create
// call twice (same ID and same (Scope, Key, Value)) — this is the
// idempotent case: it succeeds and returns the same tag, not ErrConflict.
// See docs/adr/0019-tag-identity-and-get-or-create.md.
func testCreateSameIDSameIdentity(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))

	again := sampleTag("t1")
	if err := r.Create(context.Background(), again); err != nil {
		t.Fatalf("Create with the same ID and identity returned %v, want nil", err)
	}
	if again.ID != "t1" {
		t.Fatalf("Create with the same ID and identity returned ID %q, want %q", again.ID, "t1")
	}
}

// testCreateGetOrCreateHit is the case docs/adr/0019 exists to fix: a
// second Create with a different caller-supplied ID but the same
// (Scope, Key, Value) must resolve to the first tag, not create a
// duplicate.
func testCreateGetOrCreateHit(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	first := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	mustCreate(t, r, first)

	second := &domain.Tag{ID: "t2", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata, Category: "should be discarded"}
	if err := r.Create(context.Background(), second); err != nil {
		t.Fatalf("Create with a different ID but the same identity returned %v, want nil", err)
	}
	if second.ID != "t1" {
		t.Fatalf("Create with a different ID but the same identity returned ID %q, want the existing tag's ID %q", second.ID, "t1")
	}
	if second.Category != "" {
		t.Fatalf("Create with a different ID but the same identity returned Category %q, want the existing tag's Category %q", second.Category, "")
	}

	if _, err := r.Get(context.Background(), "t2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get for the discarded ID t2 returned %v, want ErrNotFound", err)
	}
}

// testCreateIDCollision covers two different tags whose caller-supplied
// IDs happen to collide — a genuine conflict, not a get-or-create hit.
func testCreateIDCollision(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))

	colliding := &domain.Tag{ID: "t1", Key: "mood", Value: "dark", Scope: domain.TagScopeMetadata}
	err := r.Create(context.Background(), colliding)
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Create with a colliding ID but a different identity returned %v, want ErrConflict", err)
	}
}

// testCreateDifferentScopeIsDistinct covers Scope being part of identity:
// the same Key/Value under a different Scope is a different tag, not a
// get-or-create hit.
func testCreateDifferentScopeIsDistinct(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	metadata := sampleTag("t1")
	mustCreate(t, r, metadata)

	user := &domain.Tag{ID: "t2", Key: metadata.Key, Value: metadata.Value, Scope: domain.TagScopeUser}
	if err := r.Create(context.Background(), user); err != nil {
		t.Fatalf("Create with the same key/value under a different scope returned %v, want nil", err)
	}
	if user.ID != "t2" {
		t.Fatalf("Create with the same key/value under a different scope returned ID %q, want %q (a new tag, not a get-or-create hit)", user.ID, "t2")
	}
}

func testUpdate(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	tag := sampleTag("t1")
	mustCreate(t, r, tag)

	tag.Value = "gonzo"
	if err := r.Update(context.Background(), tag); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := r.Get(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != "gonzo" {
		t.Fatalf("Get after Update returned Value %q, want %q", got.Value, "gonzo")
	}
}

func testUpdateMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Update(context.Background(), sampleTag("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing tag returned %v, want ErrNotFound", err)
	}
}

// testUpdateRenameConflict covers renaming a tag's Key/Value/Scope into an
// identity a different, live tag already owns — see
// docs/adr/0019-tag-identity-and-get-or-create.md.
func testUpdateRenameConflict(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	taken := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	mustCreate(t, r, taken)
	other := &domain.Tag{ID: "t2", Key: "mood", Value: "dark", Scope: domain.TagScopeMetadata}
	mustCreate(t, r, other)

	rename := &domain.Tag{ID: "t2", Key: taken.Key, Value: taken.Value, Scope: taken.Scope}
	err := r.Update(ctx, rename)
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("Update renaming into an identity owned by another tag returned %v, want ErrConflict", err)
	}

	got, err := r.Get(ctx, "t2")
	if err != nil {
		t.Fatalf("Get after failed rename returned error: %v", err)
	}
	if got.Value != "dark" {
		t.Fatalf("Get after failed rename returned Value %q, want the original %q (rename must not partially apply)", got.Value, "dark")
	}
}

// testUpdateRenameFreesOldIdentity covers a successful rename: the old
// identity becomes available for a brand new tag to claim.
func testUpdateRenameFreesOldIdentity(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	tag := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	mustCreate(t, r, tag)
	freedKey, freedValue, freedScope := tag.Key, tag.Value, tag.Scope

	renamed := &domain.Tag{ID: "t1", Key: "mood", Value: "dark", Scope: domain.TagScopeMetadata}
	if err := r.Update(ctx, renamed); err != nil {
		t.Fatalf("Update renaming to a free identity returned error: %v", err)
	}

	reclaimed := &domain.Tag{ID: "t2", Key: freedKey, Value: freedValue, Scope: freedScope}
	if err := r.Create(ctx, reclaimed); err != nil {
		t.Fatalf("Create reusing the freed identity returned %v, want nil", err)
	}
	if reclaimed.ID != "t2" {
		t.Fatalf("Create reusing the freed identity returned ID %q, want the new tag %q, not a stale get-or-create hit", reclaimed.ID, "t2")
	}
}

func testDelete(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))

	if err := r.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := r.Get(context.Background(), "t1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing tag returned %v, want ErrNotFound", err)
	}
}

// testDeleteFreesIdentity covers a deleted tag's identity becoming
// available for a brand new tag to claim — see
// docs/adr/0019-tag-identity-and-get-or-create.md.
func testDeleteFreesIdentity(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	tag := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	mustCreate(t, r, tag)

	if err := r.Delete(ctx, "t1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	reclaimed := &domain.Tag{ID: "t2", Key: tag.Key, Value: tag.Value, Scope: tag.Scope}
	if err := r.Create(ctx, reclaimed); err != nil {
		t.Fatalf("Create reusing the freed identity returned %v, want nil", err)
	}
	if reclaimed.ID != "t2" {
		t.Fatalf("Create reusing the freed identity returned ID %q, want the new tag %q, not a stale get-or-create hit", reclaimed.ID, "t2")
	}
}

func testListPaginates(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 5 {
		id := fmt.Sprintf("t%d", i)
		mustCreate(t, r, sampleTag(id))
		want[id] = true
	}

	got := map[string]bool{}
	pageToken := ""
	for {
		tags, next, err := r.List(ctx, 2, pageToken)
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		for _, tag := range tags {
			got[tag.ID] = true
		}
		if next == "" {
			break
		}
		pageToken = next
	}

	if len(got) != len(want) {
		t.Fatalf("List across pages returned %d tags, want %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("List across pages missing tag %q", id)
		}
	}
}

// sampleTag builds a Tag whose identity (Scope, Key, Value) is unique to
// id, so distinct ids never collide by accident under get-or-create
// semantics — see docs/adr/0019-tag-identity-and-get-or-create.md. Tests
// that specifically want two ids to share one identity build that
// identity explicitly instead of using this helper twice.
func sampleTag(id string) *domain.Tag {
	return &domain.Tag{ID: id, Key: "genre", Value: "action-" + id, Scope: domain.TagScopeMetadata}
}

func testDeleteBatch(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))
	mustCreate(t, r, sampleTag("t2"))
	mustCreate(t, r, sampleTag("t3"))

	if err := r.DeleteBatch(context.Background(), []string{"t1", "t2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := r.Get(context.Background(), "t1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for t1 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "t2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after DeleteBatch for t2 returned %v, want ErrNotFound", err)
	}
	if _, err := r.Get(context.Background(), "t3"); err != nil {
		t.Fatalf("DeleteBatch removed t3, which wasn't in the batch: %v", err)
	}
}

func testDeleteBatchRollsBackOnMissing(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	mustCreate(t, r, sampleTag("t1"))

	err := r.DeleteBatch(context.Background(), []string{"t1", "missing"})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := r.Get(context.Background(), "t1"); err != nil {
		t.Fatalf("DeleteBatch removed t1 despite rolling back: %v", err)
	}
}

// testDeleteBatchFreesIdentities covers every deleted tag's identity
// becoming available for reuse, same as a single Delete — see
// docs/adr/0019-tag-identity-and-get-or-create.md.
func testDeleteBatchFreesIdentities(t *testing.T, newRepo NewRepositoryFunc) {
	r := newRepo(t)
	ctx := context.Background()
	tag := &domain.Tag{ID: "t1", Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
	mustCreate(t, r, tag)

	if err := r.DeleteBatch(ctx, []string{"t1"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	reclaimed := &domain.Tag{ID: "t2", Key: tag.Key, Value: tag.Value, Scope: tag.Scope}
	if err := r.Create(ctx, reclaimed); err != nil {
		t.Fatalf("Create reusing the freed identity returned %v, want nil", err)
	}
	if reclaimed.ID != "t2" {
		t.Fatalf("Create reusing the freed identity returned ID %q, want the new tag %q, not a stale get-or-create hit", reclaimed.ID, "t2")
	}
}
