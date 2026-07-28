package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// newOrganizerFixture wires a fakeMediaFileRepository/fakeItemRepository
// pair with one Item/MediaFile row (source file written at srcPath with
// content), one fakeTemplateDataBuilder registered for
// domain.ContentTypeMusic, and an OrganizeConfig rooted at destRoot using
// the template "{{.ArtistName}}/{{.TrackTitle}}{{.Ext}}" — every organizer
// test below builds on this same shape, varying only the Organizer's
// options and, occasionally, what's already sitting at the destination.
func newOrganizerFixture(t *testing.T, content string) (mediaFiles *fakeMediaFileRepository, items *fakeItemRepository, configs map[domain.ContentType]service.OrganizeConfig, srcPath, destRoot, wantDest string) {
	t.Helper()

	srcDir := t.TempDir()
	destRoot = t.TempDir()
	srcPath = filepath.Join(srcDir, "source.flac")
	if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing source file: %v", err)
	}

	items = newFakeItemRepository()
	item := &domain.Item{
		ID:             "item1",
		ContentType:    domain.ContentTypeMusic,
		LibraryEntryID: "entry1",
		Title:          "Test Track",
		Status:         domain.ItemStatusImported,
	}
	if err := items.Create(context.Background(), item); err != nil {
		t.Fatalf("creating fixture item: %v", err)
	}

	mediaFiles = newFakeMediaFileRepository()
	mf := &domain.MediaFile{ID: "mf1", ItemID: item.ID, Path: srcPath}
	if err := mediaFiles.Create(context.Background(), mf); err != nil {
		t.Fatalf("creating fixture media file: %v", err)
	}

	configs = map[domain.ContentType]service.OrganizeConfig{
		domain.ContentTypeMusic: {Root: destRoot, Template: "{{.ArtistName}}/{{.TrackTitle}}{{.Ext}}"},
	}
	wantDest = filepath.Join(destRoot, "Test Artist", "Test Track.flac")
	return mediaFiles, items, configs, srcPath, destRoot, wantDest
}

func newFixtureBuilder() *fakeTemplateDataBuilder {
	return &fakeTemplateDataBuilder{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		data:         map[string]any{"ArtistName": "Test Artist", "TrackTitle": "Test Track"},
	}
}

func TestOrganizer_Organize_SameFilesystemRename(t *testing.T) {
	mediaFiles, items, configs, srcPath, _, wantDest := newOrganizerFixture(t, "hello world")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	got, err := organizer.Organize(context.Background(), "mf1")
	if err != nil {
		t.Fatalf("Organize returned error: %v", err)
	}
	if got.Path != wantDest {
		t.Fatalf("Organize returned Path %q, want %q", got.Path, wantDest)
	}

	if _, err := os.Stat(srcPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original file at %q still exists after rename", srcPath)
	}
	gotContent, err := os.ReadFile(wantDest)
	if err != nil {
		t.Fatalf("reading destination file: %v", err)
	}
	if string(gotContent) != "hello world" {
		t.Fatalf("destination content = %q, want %q", gotContent, "hello world")
	}

	stored, err := mediaFiles.Get(context.Background(), "mf1")
	if err != nil {
		t.Fatalf("Get after Organize returned error: %v", err)
	}
	if stored.Path != wantDest {
		t.Fatalf("stored media file Path = %q, want %q", stored.Path, wantDest)
	}
}

func TestOrganizer_Organize_ForcedCrossFilesystemFallback(t *testing.T) {
	mediaFiles, items, configs, srcPath, _, wantDest := newOrganizerFixture(t, "hello world")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())

	renameCalled := false
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil,
		service.WithRenameFunc(func(_, _ string) error {
			renameCalled = true
			return errors.New("simulated cross-device rename failure")
		}),
	)

	got, err := organizer.Organize(context.Background(), "mf1")
	if err != nil {
		t.Fatalf("Organize returned error: %v", err)
	}
	if !renameCalled {
		t.Fatal("Organize never attempted rename before falling back to copy")
	}
	if got.Path != wantDest {
		t.Fatalf("Organize returned Path %q, want %q", got.Path, wantDest)
	}

	if _, err := os.Stat(srcPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original file at %q still exists after copy-fallback move", srcPath)
	}
	gotContent, err := os.ReadFile(wantDest)
	if err != nil {
		t.Fatalf("reading destination file: %v", err)
	}
	if string(gotContent) != "hello world" {
		t.Fatalf("destination content = %q, want %q", gotContent, "hello world")
	}
}

func TestOrganizer_Organize_CollisionAtDestinationRefusesToOverwrite(t *testing.T) {
	mediaFiles, items, configs, srcPath, _, wantDest := newOrganizerFixture(t, "hello world")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	if err := os.MkdirAll(filepath.Dir(wantDest), 0o755); err != nil {
		t.Fatalf("preparing destination directory: %v", err)
	}
	if err := os.WriteFile(wantDest, []byte("pre-existing, must not be overwritten"), 0o644); err != nil {
		t.Fatalf("writing pre-existing destination file: %v", err)
	}

	_, err := organizer.Organize(context.Background(), "mf1")
	if !errors.Is(err, service.ErrDestinationExists) {
		t.Fatalf("Organize returned %v, want ErrDestinationExists", err)
	}

	destContent, readErr := os.ReadFile(wantDest)
	if readErr != nil {
		t.Fatalf("reading destination file: %v", readErr)
	}
	if string(destContent) != "pre-existing, must not be overwritten" {
		t.Fatalf("destination file was overwritten: got %q", destContent)
	}

	srcContent, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("original file at %q was removed on collision: %v", srcPath, err)
	}
	if string(srcContent) != "hello world" {
		t.Fatalf("original file content changed: got %q", srcContent)
	}

	stored, err := mediaFiles.Get(context.Background(), "mf1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.Path != srcPath {
		t.Fatalf("stored media file Path changed to %q on a refused collision", stored.Path)
	}
}

func TestOrganizer_Organize_ForcedPartialCopyThenCrashNeverDeletesOriginal(t *testing.T) {
	mediaFiles, items, configs, srcPath, _, _ := newOrganizerFixture(t, "hello world, this is the full original content")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())

	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil,
		service.WithRenameFunc(func(_, _ string) error {
			return errors.New("simulated cross-device rename failure")
		}),
		service.WithCopyFunc(func(_, dst string) error {
			// Simulate a copy that is silently truncated (writes fewer
			// bytes than the source and still reports success) rather
			// than the process crashing mid-write — both leave a
			// short/incomplete file at dst with no error of their own,
			// which is exactly what move's post-copy size verification
			// must catch before it would otherwise delete src.
			return os.WriteFile(dst, []byte("incomplete"), 0o644)
		}),
	)

	_, err := organizer.Organize(context.Background(), "mf1")
	if err == nil {
		t.Fatal("Organize succeeded despite an incomplete copy, want an error")
	}

	srcContent, readErr := os.ReadFile(srcPath)
	if readErr != nil {
		t.Fatalf("original file at %q was removed after an incomplete copy: %v", srcPath, readErr)
	}
	if string(srcContent) != "hello world, this is the full original content" {
		t.Fatalf("original file content changed: got %q", srcContent)
	}

	stored, err := mediaFiles.Get(context.Background(), "mf1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.Path != srcPath {
		t.Fatalf("stored media file Path changed to %q after a failed organize", stored.Path)
	}
}

func TestOrganizer_Organize_NoConfigForContentType(t *testing.T) {
	mediaFiles, items, _, _, _, _ := newOrganizerFixture(t, "hello world")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())
	organizer := service.NewOrganizer(mediaFiles, items, registry, map[domain.ContentType]service.OrganizeConfig{}, nil)

	if _, err := organizer.Organize(context.Background(), "mf1"); err == nil {
		t.Fatal("Organize succeeded with no OrganizeConfig registered for the item's content type")
	}
}

func TestOrganizer_Organize_MediaFileNotFound(t *testing.T) {
	mediaFiles, items, configs, _, _, _ := newOrganizerFixture(t, "hello world")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	if _, err := organizer.Organize(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Organize returned %v, want ErrNotFound", err)
	}
}

func TestOrganizer_Organize_ItemNotFound(t *testing.T) {
	mediaFiles, items, configs, _, _, _ := newOrganizerFixture(t, "hello world")
	orphan := &domain.MediaFile{ID: "mf-orphan", ItemID: "no-such-item", Path: "/tmp/whatever"}
	if err := mediaFiles.Create(context.Background(), orphan); err != nil {
		t.Fatalf("creating orphan media file: %v", err)
	}
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	if _, err := organizer.Organize(context.Background(), "mf-orphan"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Organize returned %v, want ErrNotFound", err)
	}
}

func TestOrganizer_Organize_TemplateDataBuilderError(t *testing.T) {
	mediaFiles, items, _, _, destRoot, _ := newOrganizerFixture(t, "hello world")
	wantErr := errors.New("build failed")
	builder := &fakeTemplateDataBuilder{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, err: wantErr}
	registry := service.NewTemplateDataBuilderRegistry(builder)
	configs := map[domain.ContentType]service.OrganizeConfig{
		domain.ContentTypeMusic: {Root: destRoot, Template: "{{.ArtistName}}/{{.TrackTitle}}{{.Ext}}"},
	}
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	if _, err := organizer.Organize(context.Background(), "mf1"); !errors.Is(err, wantErr) {
		t.Fatalf("Organize returned %v, want %v", err, wantErr)
	}
}

func TestOrganizer_Organize_InvalidTemplateSyntax(t *testing.T) {
	mediaFiles, items, _, _, destRoot, _ := newOrganizerFixture(t, "hello world")
	registry := service.NewTemplateDataBuilderRegistry(newFixtureBuilder())
	configs := map[domain.ContentType]service.OrganizeConfig{
		domain.ContentTypeMusic: {Root: destRoot, Template: "{{.ArtistName"},
	}
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	if _, err := organizer.Organize(context.Background(), "mf1"); err == nil {
		t.Fatal("Organize succeeded with an unparseable template, want an error")
	}
}

func TestOrganizer_Organize_DestinationOutsideRootIsRefused(t *testing.T) {
	mediaFiles, items, _, _, destRoot, _ := newOrganizerFixture(t, "hello world")
	builder := &fakeTemplateDataBuilder{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		data:         map[string]any{"ArtistName": "../../escaped", "TrackTitle": "Test Track"},
	}
	registry := service.NewTemplateDataBuilderRegistry(builder)
	configs := map[domain.ContentType]service.OrganizeConfig{
		domain.ContentTypeMusic: {Root: destRoot, Template: "{{.ArtistName}}/{{.TrackTitle}}{{.Ext}}"},
	}
	organizer := service.NewOrganizer(mediaFiles, items, registry, configs, nil)

	_, err := organizer.Organize(context.Background(), "mf1")
	if !errors.Is(err, service.ErrDestinationOutsideRoot) {
		t.Fatalf("Organize returned %v, want ErrDestinationOutsideRoot", err)
	}
}
