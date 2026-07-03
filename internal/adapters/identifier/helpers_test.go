package identifier_test

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
)

// --- stub repositories ---

type stubMediaFileRepo struct {
	byHash map[string]*domain.MediaFile
}

func (s *stubMediaFileRepo) GetByOSHash(_ context.Context, hash string) (*domain.MediaFile, error) {
	mf, ok := s.byHash[hash]
	if !ok {
		return nil, errs.ErrNotFound
	}
	return mf, nil
}

func (s *stubMediaFileRepo) GetByItemID(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (s *stubMediaFileRepo) GetByPath(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (s *stubMediaFileRepo) Save(_ context.Context, _ *domain.MediaFile) error { return nil }
func (s *stubMediaFileRepo) Delete(_ context.Context, _ string) error          { return nil }

type stubItemRepo struct {
	byID     map[string]*domain.Item
	bySearch []*domain.Item // returned for any List call
}

func (s *stubItemRepo) Get(_ context.Context, id string) (*domain.Item, error) {
	item, ok := s.byID[id]
	if !ok {
		return nil, errs.ErrNotFound
	}
	return item, nil
}

func (s *stubItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	return s.bySearch, len(s.bySearch), nil
}

func (s *stubItemRepo) Save(_ context.Context, _ *domain.Item) error { return nil }
func (s *stubItemRepo) Delete(_ context.Context, _ string) error     { return nil }
func (s *stubItemRepo) DeleteByGroup(_ context.Context, _ string) error {
	return nil
}

func (s *stubItemRepo) DeleteByLibraryEntry(_ context.Context, _ string) error {
	return nil
}

func (s *stubItemRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return nil, errs.ErrNotFound
}

type stubExternalIDRepo struct {
	entries map[string]string // "entityType:source:value" → internalID
}

func (s *stubExternalIDRepo) FindEntity(_ context.Context, entityType, source, value string) (string, error) {
	key := entityType + ":" + source + ":" + value
	id, ok := s.entries[key]
	if !ok {
		return "", errs.ErrNotFound
	}
	return id, nil
}
