package service_test

import (
	"context"
	"errors"
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

func TestScanService_Trigger(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-1"}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{
		{Path: "/media/a.flac", Size: 1},
		{Path: "/media/b.flac", Size: 2},
	}}
	svc := service.NewScanService(pub, walker, true, false)

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
}

func TestScanService_Trigger_WalkerError(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeScanJobPublisher{}
	walker := &fakeFileWalker{err: wantErr}
	svc := service.NewScanService(pub, walker, false, false)

	if _, err := svc.Trigger(context.Background(), "/media"); !errors.Is(err, wantErr) {
		t.Fatalf("Trigger returned %v, want wrapping %v", err, wantErr)
	}
}

func TestScanService_Trigger_PublisherError(t *testing.T) {
	wantErr := errors.New("boom")
	pub := &fakeScanJobPublisher{err: wantErr}
	walker := &fakeFileWalker{files: []ports.DiscoveredFile{{Path: "/media/a.flac"}}}
	svc := service.NewScanService(pub, walker, false, false)

	if _, err := svc.Trigger(context.Background(), "/media"); !errors.Is(err, wantErr) {
		t.Fatalf("Trigger returned %v, want %v", err, wantErr)
	}
}

func TestScanService_Trigger_EmptyDirectory(t *testing.T) {
	pub := &fakeScanJobPublisher{returnID: "job-empty"}
	walker := &fakeFileWalker{files: nil}
	svc := service.NewScanService(pub, walker, false, false)

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
