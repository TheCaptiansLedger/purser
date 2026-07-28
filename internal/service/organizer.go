package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"text/template"
)

// ErrDestinationExists is returned when the computed destination path
// already has something at it. The Organizer never overwrites — a
// template bug or a leftover from an interrupted previous organize could
// otherwise destroy an unrelated file. Per
// docs/technical/pipeline-music-organizer.md, what a caller does with this
// error differs by trigger: the manual RPC path (built in a later
// milestone) surfaces it directly; the automatic path (also a later
// milestone) logs it and leaves the file where it is, never blocking the
// overall persist operation. That differential handling lives in whoever
// calls Organize, not here.
var ErrDestinationExists = errors.New("organizer: destination already exists")

// ErrDestinationOutsideRoot is returned when the rendered template
// produces a path that escapes its configured OrganizeConfig.Root (e.g.
// a "../" segment from unexpected metadata). The Organizer refuses rather
// than writing outside the tree an operator configured.
var ErrDestinationOutsideRoot = errors.New("organizer: rendered destination escapes configured root")

// OrganizeConfig pairs the base directory a content type organizes into
// with the relative Go text/template naming pattern joined onto it — this
// package's own plain copy of config.OrganizeConfig's shape, so it stays
// free of any internal/config dependency (the composition root converts
// config.Pipeline.Organize into a map[domain.ContentType]OrganizeConfig
// when constructing an Organizer), consistent with every other service
// (see RootContentType/ScanService).
type OrganizeConfig struct {
	// Root is the base directory this content type organizes into.
	Root string

	// Template is a relative Go text/template pattern, joined onto Root,
	// rendered against whatever map a ports.TemplateDataBuilder produces
	// for the Item being organized.
	Template string
}

// RenameFunc moves oldpath to newpath, matching os.Rename's signature —
// the seam Organizer's rename step calls through, overridable via
// WithRenameFunc so tests can force the copy-fallback path deterministically
// without needing two real filesystems.
type RenameFunc func(oldpath, newpath string) error

// CopyFunc copies the file at src to dst, matching the shape Organizer's
// copy-fallback step calls through, overridable via WithCopyFunc so tests
// can force partial/failed-copy scenarios deterministically.
type CopyFunc func(src, dst string) error

// Organizer is the generic, content-type-agnostic mechanics of
// docs/adr/0024-pipeline-core.md's Organizer stage: given a MediaFile, it
// fetches the MediaFile's Item, dispatches to the ContentTypes()-registered
// ports.TemplateDataBuilder for the Item's content type, renders that
// content type's configured Go text/template against the resulting map,
// computes the destination path, moves the file, and updates
// MediaFile.Path. It has no idea what any of the rendered map's keys mean
// — see docs/technical/pipeline-music-organizer.md.
type Organizer struct {
	mediaFiles   ports.MediaFileRepository
	items        ports.ItemRepository
	templateData ports.TemplateDataBuilderResolver
	configs      map[domain.ContentType]OrganizeConfig
	logger       *slog.Logger
	rename       RenameFunc
	copy         CopyFunc
}

// OrganizerOption customizes an Organizer constructed via NewOrganizer.
type OrganizerOption func(*Organizer)

// WithRenameFunc overrides the default (os.Rename) rename step.
func WithRenameFunc(fn RenameFunc) OrganizerOption {
	return func(o *Organizer) { o.rename = fn }
}

// WithCopyFunc overrides the default copy-fallback step.
func WithCopyFunc(fn CopyFunc) OrganizerOption {
	return func(o *Organizer) { o.copy = fn }
}

// NewOrganizer constructs an Organizer backed by mediaFiles, items, and
// templateData, using configs to resolve each content type's destination
// root and naming template. logger defaults to slog.Default() if nil, per
// docs/adr/0007-telemetry.md's no-cost-to-opt-out convention.
func NewOrganizer(mediaFiles ports.MediaFileRepository, items ports.ItemRepository, templateData ports.TemplateDataBuilderResolver, configs map[domain.ContentType]OrganizeConfig, logger *slog.Logger, opts ...OrganizerOption) *Organizer {
	if logger == nil {
		logger = slog.Default()
	}
	o := &Organizer{
		mediaFiles:   mediaFiles,
		items:        items,
		templateData: templateData,
		configs:      configs,
		logger:       logger.With("component", "service.organizer"),
		rename:       os.Rename,
		copy:         copyFile,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Organize fetches the MediaFile identified by mediaFileID, resolves its
// Item and that Item's content-type naming data, renders the configured
// template, moves the file to the computed destination, and persists the
// MediaFile's new Path. A collision at the destination
// (ErrDestinationExists) and every other failure leave the MediaFile and
// the file on disk untouched — Path is only ever updated after the move
// itself has fully succeeded.
func (o *Organizer) Organize(ctx context.Context, mediaFileID string) (*domain.MediaFile, error) {
	mf, err := o.mediaFiles.Get(ctx, mediaFileID)
	if err != nil {
		return nil, fmt.Errorf("organizer: getting media file %q: %w", mediaFileID, err)
	}

	item, err := o.items.Get(ctx, mf.ItemID)
	if err != nil {
		return nil, fmt.Errorf("organizer: getting item %q: %w", mf.ItemID, err)
	}

	cfg, ok := o.configs[item.ContentType]
	if !ok {
		return nil, fmt.Errorf("organizer: no organize config for content type %q", item.ContentType)
	}

	data, err := o.templateData.BuildTemplateData(ctx, item.ContentType, item)
	if err != nil {
		return nil, fmt.Errorf("organizer: building template data for item %q: %w", item.ID, err)
	}
	if data == nil {
		data = map[string]any{}
	}
	data["Ext"] = filepath.Ext(mf.Path)

	dest, err := renderDestination(cfg, data)
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Lstat(dest); statErr == nil {
		return nil, fmt.Errorf("%w: %s", ErrDestinationExists, dest)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("organizer: checking destination %q: %w", dest, statErr)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return nil, fmt.Errorf("organizer: creating destination directory for %q: %w", dest, err)
	}

	if err := o.move(mf.Path, dest); err != nil {
		return nil, err
	}

	mf.Path = dest
	if err := o.mediaFiles.Update(ctx, mf); err != nil {
		return nil, fmt.Errorf("organizer: updating media file %q path: %w", mf.ID, err)
	}

	o.logger.InfoContext(ctx, "organized media file", "media_file.id", mf.ID, "media_file.path", mf.Path)
	return mf, nil
}

// renderDestination renders cfg.Template against data and joins the result
// onto cfg.Root, refusing (ErrDestinationOutsideRoot) if the rendered path
// would land outside Root — a defense against unexpected metadata (e.g. an
// artist name containing "../") relocating a file outside the operator's
// configured tree.
func renderDestination(cfg OrganizeConfig, data map[string]any) (string, error) {
	tmpl, err := template.New("organize").Parse(cfg.Template)
	if err != nil {
		return "", fmt.Errorf("organizer: parsing template %q: %w", cfg.Template, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("organizer: rendering template %q: %w", cfg.Template, err)
	}

	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return "", fmt.Errorf("organizer: resolving root %q: %w", cfg.Root, err)
	}

	dest := filepath.Join(root, buf.String())
	if dest != root && !strings.HasPrefix(dest, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrDestinationOutsideRoot, dest)
	}
	return dest, nil
}

// move relocates src to dest: os.Rename first (the cheap, same-filesystem
// path), falling back to copy-then-verify-then-delete-original on any
// rename failure — not gated on the specific cross-device error, since
// that error is platform-specific (syscall.EXDEV doesn't exist on the
// Windows build target this project also ships) and falling back
// unconditionally costs nothing extra: a rename failure for some other
// reason (e.g. a permissions problem) simply fails again at the copy step
// with an equivalent error. The original is never removed before the copy
// is verified (by size) to have landed intact — a partial copy followed by
// deleting the source is a real data-loss path, not an acceptable
// shortcut, per docs/technical/pipeline-music-organizer.md.
func (o *Organizer) move(src, dest string) error {
	if err := o.rename(src, dest); err == nil {
		return nil
	}

	if err := o.copy(src, dest); err != nil {
		return fmt.Errorf("organizer: copying %q to %q: %w", src, dest, err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("organizer: verifying copy of %q: %w", src, err)
	}
	destInfo, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("organizer: verifying copy at %q: %w", dest, err)
	}
	if srcInfo.Size() != destInfo.Size() {
		return fmt.Errorf("organizer: copy of %q to %q is incomplete: got %d bytes, want %d", src, dest, destInfo.Size(), srcInfo.Size())
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("organizer: removing original %q after verified copy: %w", src, err)
	}
	return nil
}

// copyFile is the default CopyFunc: a straightforward whole-file copy,
// fsync'd before returning so the verification step in move can trust
// dest's size reflects what's actually durable on disk.
func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // src is a MediaFile's own recorded path, not user input
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst) //nolint:gosec // dst is computed from operator-configured OrganizeConfig.Root + a rendered template, already containment-checked by renderDestination
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
