package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// UnmatchedFileService orchestrates the review queue's Get/List/Resolve
// surface. Resolve is docs/adr/0015-deletion-impact-and-composing-services.md's
// composing-service exception applied to a pipeline queue entry rather
// than a deletion-impact flow: it's the one case allowed to depend on
// more than ports.UnmatchedFileRepository, since resolving a queue entry
// genuinely means looking across UnmatchedFile, Item (to validate the
// match target exists), and MediaFile (to create the real link) — no
// single entity's port can answer that about itself without violating
// docs/adr/0001-hexagonal-architecture.md's one-port-one-entity boundary.
// See docs/adr/0024-pipeline-core.md.
type UnmatchedFileService struct {
	repo       ports.UnmatchedFileRepository
	items      ports.ItemRepository
	mediaFiles ports.MediaFileRepository
	persister  ports.PersisterResolver
}

// NewUnmatchedFileService constructs an UnmatchedFileService backed by
// repo, items, mediaFiles, and persister (AcceptCandidate's dispatch to a
// content type's Persister).
func NewUnmatchedFileService(repo ports.UnmatchedFileRepository, items ports.ItemRepository, mediaFiles ports.MediaFileRepository, persister ports.PersisterResolver) *UnmatchedFileService {
	return &UnmatchedFileService{repo: repo, items: items, mediaFiles: mediaFiles, persister: persister}
}

// Get returns the UnmatchedFile with the given id, or ports.ErrNotFound.
func (s *UnmatchedFileService) Get(ctx context.Context, id string) (*domain.UnmatchedFile, error) {
	return s.repo.Get(ctx, id)
}

// List returns a page of UnmatchedFiles, optionally filtered by status
// (empty status means no filter), using opaque cursor pagination.
func (s *UnmatchedFileService) List(ctx context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error) {
	return s.repo.List(ctx, status, pageSize, pageToken)
}

// ListGroup returns every UnmatchedFile sharing groupKey — read-only,
// wraps ListByGroupKey. This is how the review UI fetches "every file in
// this album" to render one card and build the ID list for dismissal. See
// docs/technical/pipeline-unmatchedfile-grouping.md.
func (s *UnmatchedFileService) ListGroup(ctx context.Context, groupKey string) ([]*domain.UnmatchedFile, error) {
	return s.repo.ListByGroupKey(ctx, groupKey)
}

// DismissBatch sets Status=dismissed on every UnmatchedFile identified by
// ids and persists all of them in one UpdateBatch call — all-or-nothing,
// per docs/adr/0016-bulk-operations.md. The caller (the UI, via ListGroup)
// supplies the exact ID list; DismissBatch does no GroupKey lookup of its
// own. Returns ports.ErrNotFound if any id doesn't exist.
func (s *UnmatchedFileService) DismissBatch(ctx context.Context, ids []string) ([]*domain.UnmatchedFile, error) {
	us := make([]*domain.UnmatchedFile, 0, len(ids))
	for _, id := range ids {
		u, err := s.repo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		u.Status = domain.UnmatchedFileStatusDismissed
		us = append(us, u)
	}
	if err := s.repo.UpdateBatch(ctx, us); err != nil {
		return nil, err
	}
	return us, nil
}

// Resolve resolves the UnmatchedFile identified by id one of two ways.
// Exactly one of itemID/dismiss is expected to be set — the caller (the
// Connect handler, translating a proto oneof) is responsible for that
// shape; a request with itemID empty and dismiss false returns a
// *domain.ValidationError, the same error shape Validate() produces, so
// it reaches the client as CodeInvalidArgument via the shared
// error-mapping helper rather than a silent no-op.
//
//   - dismiss: sets Status=dismissed and persists it — kept, not deleted,
//     so a later rescan's "already known" short-circuit (which looks up by
//     hash regardless of status) doesn't silently re-queue it. Returns
//     (nil, the updated UnmatchedFile, nil).
//   - itemID: validates the Item exists, creates a real MediaFile linking
//     the file's path/hashes to it (the same server-generated-ID/Validate/
//     Create path as MediaFileService.Create), then deletes the
//     UnmatchedFile — a resolved entry leaves the review queue. Returns
//     (the new MediaFile, nil, nil).
//
// Returns ports.ErrNotFound if the UnmatchedFile or (for a match) the
// Item doesn't exist.
func (s *UnmatchedFileService) Resolve(ctx context.Context, id, itemID string, dismiss bool) (*domain.MediaFile, *domain.UnmatchedFile, error) {
	uf, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	if dismiss {
		uf.Status = domain.UnmatchedFileStatusDismissed
		if err := s.repo.Update(ctx, uf); err != nil {
			return nil, nil, err
		}
		return nil, uf, nil
	}

	if itemID == "" {
		return nil, nil, &domain.ValidationError{Errors: []domain.FieldError{
			{Field: "outcome", Rule: "required_without=Dismiss", Value: ""},
		}}
	}

	if _, err := s.items.Get(ctx, itemID); err != nil {
		return nil, nil, err
	}

	mf := &domain.MediaFile{
		ID:     domain.NewID(),
		ItemID: itemID,
		Path:   uf.Path,
		Size:   uf.Size,
		OSHash: uf.OSHash,
		MD5:    uf.MD5,
		SHA1:   uf.SHA1,
		SHA512: uf.SHA512,
	}
	if err := mf.Validate(); err != nil {
		return nil, nil, err
	}
	if err := s.mediaFiles.Create(ctx, mf); err != nil {
		return nil, nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, nil, err
	}
	return mf, nil, nil
}

// AcceptCandidate persists groupKey's winning release, per
// docs/technical/pipeline-music-persist.md's "Two triggers, one Persister"
// section: externalRef matches an existing ranked MatchCandidate in the
// group's rows (the common case — a human picks any ranked candidate, not
// necessarily the top-scored one) or it's a raw external ID the pipeline
// never generated (a human already knows the correct release), in which
// case an ad-hoc domain.MatchCandidate{ExternalRef: externalRef} with no
// Tier/Score is built and persisted directly — bypassing ConfidenceScore
// entirely, since a human-supplied identifier is definitionally
// corroborated. Both paths call the exact same Persister. On success,
// every UnmatchedFile sharing groupKey leaves the review queue via
// DeleteBatch — the same "a matched entry leaves the queue by deletion"
// behavior Resolve already has for a single file. On failure, the group's
// rows are left untouched and the error is returned as-is. Returns
// ports.ErrNotFound if groupKey has no rows.
func (s *UnmatchedFileService) AcceptCandidate(ctx context.Context, groupKey, externalRef string) error {
	rows, err := s.repo.ListByGroupKey(ctx, groupKey)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ports.ErrNotFound
	}

	candidate := findCandidate(rows, externalRef)
	if candidate == nil {
		candidate = &domain.MatchCandidate{ExternalRef: externalRef}
	}

	var fingerprint *domain.Fingerprint
	for _, row := range rows {
		if row.Fingerprint != nil {
			fingerprint = row.Fingerprint
			break
		}
	}

	if err := s.persister.Persist(ctx, rows[0].ContentType, fingerprint, *candidate, rows); err != nil {
		return err
	}

	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return s.repo.DeleteBatch(ctx, ids)
}

// findCandidate searches every row's Candidates for one whose ExternalRef
// matches externalRef, returning it as-is (Tier/Score/Signals intact) —
// the common "a human picks a ranked candidate" path. Returns nil if none
// matches, meaning externalRef is a raw, human-supplied identifier.
func findCandidate(rows []*domain.UnmatchedFile, externalRef string) *domain.MatchCandidate {
	for _, row := range rows {
		for i := range row.Candidates {
			if row.Candidates[i].ExternalRef == externalRef {
				return &row.Candidates[i]
			}
		}
	}
	return nil
}
