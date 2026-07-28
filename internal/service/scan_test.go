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

type fakeScanJobPublisher struct {
	gotKind   string
	gotLabels []string
	gotParams map[string]string
	returnID  string
	err       error
}

func (f *fakeScanJobPublisher) Trigger(_ context.Context, kind string, taskLabels []string, params map[string]string) (string, error) {
	f.gotKind, f.gotLabels, f.gotParams = kind, taskLabels, params
	if f.err != nil {
		return "", f.err
	}
	return f.returnID, nil
}

type fakeFileWalker struct {
	files []ports.DiscoveredFile
	err   error
}

func (f *fakeFileWalker) Walk(_ context.Context, _ string) ([]ports.DiscoveredFile, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.files, nil
}

// fakeSidecarClassifierResolver is a minimal ports.SidecarClassifierResolver
// double. A nil/empty kindByPath classifies every path as
// ports.SidecarKindNone (the zero value), matching NoopClassifier's
// behavior — the "nothing filtered" default most Trigger tests want.
type fakeSidecarClassifierResolver struct {
	kindByPath map[string]ports.SidecarKind
	err        error
}

func (f *fakeSidecarClassifierResolver) Classify(_ context.Context, _ domain.ContentType, path string) (ports.SidecarKind, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.kindByPath[path], nil
}

func TestScanService_Trigger(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{
		{Path: "/media/a.flac", Size: 1},
		{Path: "/media/b.flac", Size: 2},
	}}
	svc := service.NewScanService(pub, walker, true, false, nil, &fakeSidecarClassifierResolver{})

	id, err := svc.Trigger(context.Background(), "/media")
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if id != "job-1" {
		t.Fatalf("Trigger returned id %q, want %q", id, "job-1")
	}
	if pub.gotKind != "scan" {
		t.Fatalf("Trigger passed kind %q, want %q", pub.gotKind, "scan")
	}
	wantLabels := []string{"/media/a.flac", "/media/b.flac"}
	if len(pub.gotLabels) != len(wantLabels) {
		t.Fatalf("Trigger passed %d task labels, want %d", len(pub.gotLabels), len(wantLabels))
	}
	for i, l := range wantLabels {
		if pub.gotLabels[i] != l {
			t.Errorf("Trigger passed label[%d] = %q, want %q", i, pub.gotLabels[i], l)
		}
	}
	if pub.gotParams["enable_md5"] != "true" {
		t.Errorf("Trigger passed params[enable_md5] = %q, want %q", pub.gotParams["enable_md5"], "true")
	}
	if pub.gotParams["enable_sha512"] != "false" {
		t.Errorf("Trigger passed params[enable_sha512] = %q, want %q", pub.gotParams["enable_sha512"], "false")
	}
	if pub.gotParams["scan_root"] != "/media" {
		t.Errorf("Trigger passed params[scan_root] = %q, want %q", pub.gotParams["scan_root"], "/media")
	}
}

func TestScanService_Trigger_WalkerError(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeScanJobPublisher{}
	walker := &fakeFileWalker{err: wantErr}
	svc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})

	if _, err := svc.Trigger(context.Background(), "/media"); !errors.Is(err, wantErr) {
		t.Fatalf("Trigger returned %v, want wrapping %v", err, wantErr)
	}
}

func TestScanService_Trigger_PublisherError(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeScanJobPublisher{err: wantErr}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/a.flac"}}}
	svc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})

	if _, err := svc.Trigger(context.Background(), "/media"); !errors.Is(err, wantErr) {
		t.Fatalf("Trigger returned %v, want %v", err, wantErr)
	}
}

func TestScanService_Trigger_EmptyDirectory(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-empty"}
	walker := &fakeFileWalker{files: nil}
	svc := service.NewScanService(pub, walker, false, false, nil, &fakeSidecarClassifierResolver{})

	id, err := svc.Trigger(context.Background(), "/media/empty")
	if err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if id != "job-empty" {
		t.Fatalf("Trigger returned id %q, want %q", id, "job-empty")
	}
	if len(pub.gotLabels) != 0 {
		t.Fatalf("Trigger passed %d task labels, want 0", len(pub.gotLabels))
	}
}

func TestScanService_Trigger_ResolvesContentTypeFromConfiguredRoot(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/incoming/a.flac"}}}
	roots := []service.RootContentType{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMusic},
		{Path: "/media/movies", ContentType: domain.ContentTypeMovie},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, &fakeSidecarClassifierResolver{})

	if _, err := svc.Trigger(context.Background(), "/media/incoming"); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if pub.gotParams["content_type"] != string(domain.ContentTypeMusic) {
		t.Fatalf("Trigger passed params[content_type] = %q, want %q", pub.gotParams["content_type"], domain.ContentTypeMusic)
	}
}

func TestScanService_Trigger_ResolvesContentTypeForWatcherTriggeredSubfolder(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/incoming/artist/album/a.flac"}}}
	roots := []service.RootContentType{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMusic},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, &fakeSidecarClassifierResolver{})

	// A watcher's debounced event fires on a settled subfolder unit, not
	// necessarily the configured root itself — Trigger must still resolve
	// its content type.
	if _, err := svc.Trigger(context.Background(), "/media/incoming/artist/album"); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if pub.gotParams["content_type"] != string(domain.ContentTypeMusic) {
		t.Fatalf("Trigger passed params[content_type] = %q, want %q", pub.gotParams["content_type"], domain.ContentTypeMusic)
	}
}

func TestScanService_Trigger_LongestPrefixWinsAmongOverlappingRoots(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/incoming/soundtracks/a.flac"}}}
	roots := []service.RootContentType{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMovie},
		{Path: "/media/incoming/soundtracks", ContentType: domain.ContentTypeMusic},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, &fakeSidecarClassifierResolver{})

	if _, err := svc.Trigger(context.Background(), "/media/incoming/soundtracks"); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if pub.gotParams["content_type"] != string(domain.ContentTypeMusic) {
		t.Fatalf("Trigger passed params[content_type] = %q, want %q (longest-prefix match)", pub.gotParams["content_type"], domain.ContentTypeMusic)
	}
}

func TestScanService_Trigger_NoMatchingRootResolvesEmptyContentType(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/other/a.flac"}}}
	roots := []service.RootContentType{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMusic},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, &fakeSidecarClassifierResolver{})

	if _, err := svc.Trigger(context.Background(), "/other"); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if pub.gotParams["content_type"] != "" {
		t.Fatalf("Trigger passed params[content_type] = %q, want empty", pub.gotParams["content_type"])
	}
}

func TestScanService_Trigger_DoesNotMatchSimilarlyNamedRoot(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/incoming-backup/a.flac"}}}
	roots := []service.RootContentType{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMusic},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, &fakeSidecarClassifierResolver{})

	// "/media/incoming-backup" must not match the "/media/incoming" root
	// as a prefix — only real path-segment descendants should match.
	if _, err := svc.Trigger(context.Background(), "/media/incoming-backup"); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if pub.gotParams["content_type"] != "" {
		t.Fatalf("Trigger passed params[content_type] = %q, want empty (no real match)", pub.gotParams["content_type"])
	}
}

func TestScanService_Trigger_ExcludesClassifiedSidecarsFromTaskLabels(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{
		{Path: "/media/incoming/01.flac"},
		{Path: "/media/incoming/cover.jpg"},
		{Path: "/media/incoming/album.nfo"},
	}}
	classifier := &fakeSidecarClassifierResolver{kindByPath: map[string]ports.SidecarKind{
		"/media/incoming/cover.jpg": ports.SidecarKindImage,
		"/media/incoming/album.nfo": ports.SidecarKindOther,
	}}
	roots := []service.RootContentType{
		{Path: "/media/incoming", ContentType: domain.ContentTypeMusic},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, classifier)

	if _, err := svc.Trigger(context.Background(), "/media/incoming"); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	wantLabels := []string{"/media/incoming/01.flac"}
	if len(pub.gotLabels) != len(wantLabels) {
		t.Fatalf("Trigger passed task labels %v, want %v", pub.gotLabels, wantLabels)
	}
	for i, l := range wantLabels {
		if pub.gotLabels[i] != l {
			t.Errorf("Trigger passed label[%d] = %q, want %q", i, pub.gotLabels[i], l)
		}
	}
}

func TestScanService_Trigger_ClassifierError(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/a.flac"}}}
	classifier := &fakeSidecarClassifierResolver{err: wantErr}
	svc := service.NewScanService(pub, walker, false, false, nil, classifier)

	if _, err := svc.Trigger(context.Background(), "/media"); !errors.Is(err, wantErr) {
		t.Fatalf("Trigger returned %v, want wrapping %v", err, wantErr)
	}
}

func TestScanService_Trigger_ResolvesContentTypeWhenConfiguredRootIsRelativeButIncomingRootIsAbsolute(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	absRoot := filepath.Join(cwd, "incoming", "music")

	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: filepath.Join(absRoot, "a.flac")}}}
	roots := []service.RootContentType{
		// Deliberately relative, as a config file could plausibly write it —
		// pkg/fswatch always hands Trigger an absolute path for a
		// watcher-triggered scan regardless, so resolution must still
		// succeed against that absolute path.
		{Path: filepath.Join("incoming", "music"), ContentType: domain.ContentTypeMusic},
	}
	svc := service.NewScanService(pub, walker, false, false, roots, &fakeSidecarClassifierResolver{})

	if _, err := svc.Trigger(context.Background(), absRoot); err != nil {
		t.Fatalf("Trigger returned error: %v", err)
	}
	if pub.gotParams["content_type"] != string(domain.ContentTypeMusic) {
		t.Fatalf("Trigger passed params[content_type] = %q, want %q (relative configured root must still resolve against an absolute incoming root)", pub.gotParams["content_type"], domain.ContentTypeMusic)
	}
}
