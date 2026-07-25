package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// UnmatchedFileService orchestrates ports.UnmatchedFileRepository — the
// review queue's Get/List surface. See docs/adr/0024-pipeline-core.md.
type UnmatchedFileService struct {
	repo ports.UnmatchedFileRepository
}

// NewUnmatchedFileService constructs an UnmatchedFileService backed by
// repo.
func NewUnmatchedFileService(repo ports.UnmatchedFileRepository) *UnmatchedFileService {
	return &UnmatchedFileService{repo: repo}
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
