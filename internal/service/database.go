package service

import (
	"context"
	"io"
	"purser/internal/ports"
)

// DatabaseService orchestrates ports.DatabaseAdmin — a thin pass-through,
// the same shape as JobService, per docs/adr/0011-api-design.md and
// docs/technical/database-backup-restore.md.
type DatabaseService struct {
	admin ports.DatabaseAdmin

	// onRestore is called once, synchronously, after a successful
	// Restore — nil-safe. cmd/purser wires this to a delayed call to the
	// same stop func() runServe already derives from
	// signal.NotifyContext, which is how a successful restore actually
	// makes the process exit for an external supervisor to restart. This
	// package stays free of any process-lifecycle or filesystem
	// concerns — that wiring belongs in the composition root.
	onRestore func()
}

// NewDatabaseService constructs a DatabaseService backed by admin.
// onRestore may be nil.
func NewDatabaseService(admin ports.DatabaseAdmin, onRestore func()) *DatabaseService {
	return &DatabaseService{admin: admin, onRestore: onRestore}
}

// Info returns the current DatabaseInfo snapshot.
func (s *DatabaseService) Info(ctx context.Context) (ports.DatabaseInfo, error) {
	return s.admin.Info(ctx)
}

// Backup writes the full backup artifact to w.
func (s *DatabaseService) Backup(ctx context.Context, w io.Writer) error {
	return s.admin.Backup(ctx, w)
}

// Restore validates and applies the backup artifact read from r,
// destroying and replacing every document currently in the database. On
// success, onRestore fires (if set) before Restore returns.
func (s *DatabaseService) Restore(ctx context.Context, r io.Reader) error {
	if err := s.admin.Restore(ctx, r); err != nil {
		return err
	}
	if s.onRestore != nil {
		s.onRestore()
	}
	return nil
}
