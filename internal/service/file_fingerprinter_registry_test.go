package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakeFileFingerprinter is a minimal ports.FileFingerprinter double for
// exercising FileFingerprinterRegistry's dispatch — every call returns a
// fixed, recognizable domain.Fingerprint so tests can tell it apart from
// service.NoopFingerprinter's always-empty behavior.
type fakeFileFingerprinter struct {
	contentTypes []domain.ContentType
	fingerprint  domain.Fingerprint
	consensus    domain.Fingerprint
	err          error
}

func (f *fakeFileFingerprinter) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeFileFingerprinter) Fingerprint(_ context.Context, _ string, _ int) (domain.Fingerprint, error) {
	if f.err != nil {
		return domain.Fingerprint{}, f.err
	}
	return f.fingerprint, nil
}

func (f *fakeFileFingerprinter) Consensus(_ context.Context, _ []domain.Fingerprint) (domain.Fingerprint, error) {
	if f.err != nil {
		return domain.Fingerprint{}, f.err
	}
	return f.consensus, nil
}

func TestFileFingerprinterRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	fixed := domain.Fingerprint{Tags: map[string]string{"ALBUM": "Fixed Album"}}
	music := &fakeFileFingerprinter{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, fingerprint: fixed}
	registry := service.NewFileFingerprinterRegistry(music)

	got, err := registry.Fingerprint(context.Background(), domain.ContentTypeMusic, "/media/a.flac", 0)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if got.Tags["ALBUM"] != "Fixed Album" {
		t.Fatalf("Fingerprint().Tags[ALBUM] = %q, want %q", got.Tags["ALBUM"], "Fixed Album")
	}
}

func TestFileFingerprinterRegistry_ConsensusDispatchesToRegisteredImplementation(t *testing.T) {
	fixed := domain.Fingerprint{Tags: map[string]string{"ALBUM": "Consensus Album"}}
	music := &fakeFileFingerprinter{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, consensus: fixed}
	registry := service.NewFileFingerprinterRegistry(music)

	got, err := registry.Consensus(context.Background(), domain.ContentTypeMusic, []domain.Fingerprint{{}})
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if got.Tags["ALBUM"] != "Consensus Album" {
		t.Fatalf("Consensus().Tags[ALBUM] = %q, want %q", got.Tags["ALBUM"], "Consensus Album")
	}
}

func TestFileFingerprinterRegistry_FallsBackToNoopWhenUnregistered(t *testing.T) {
	music := &fakeFileFingerprinter{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, fingerprint: domain.Fingerprint{Tags: map[string]string{"ALBUM": "x"}}}
	registry := service.NewFileFingerprinterRegistry(music)

	got, err := registry.Fingerprint(context.Background(), domain.ContentTypeMovie, "/media/a.mkv", 0)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if got.Tags != nil || got.Metadata != nil {
		t.Fatalf("Fingerprint() = %+v, want empty domain.Fingerprint (NoopFingerprinter fallback)", got)
	}
}

func TestFileFingerprinterRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewFileFingerprinterRegistry()

	got, err := registry.Consensus(context.Background(), domain.ContentTypeMusic, []domain.Fingerprint{{}})
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if got.Tags != nil || got.Metadata != nil {
		t.Fatalf("Consensus() = %+v, want empty domain.Fingerprint", got)
	}
}

func TestFileFingerprinterRegistry_PropagatesImplementationError(t *testing.T) {
	wantErr := errors.New("boom")
	music := &fakeFileFingerprinter{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, err: wantErr}
	registry := service.NewFileFingerprinterRegistry(music)

	_, err := registry.Fingerprint(context.Background(), domain.ContentTypeMusic, "/media/a.flac", 0)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Fingerprint returned %v, want wrapping %v", err, wantErr)
	}

	_, err = registry.Consensus(context.Background(), domain.ContentTypeMusic, []domain.Fingerprint{{}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Consensus returned %v, want wrapping %v", err, wantErr)
	}
}

func TestNoopFingerprinter_ReturnsEmptyFingerprint(t *testing.T) {
	f := service.NoopFingerprinter{}

	got, err := f.Fingerprint(context.Background(), "/media/a.flac", 0)
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if got.Tags != nil || got.Metadata != nil {
		t.Fatalf("Fingerprint() = %+v, want empty domain.Fingerprint", got)
	}

	got, err = f.Consensus(context.Background(), []domain.Fingerprint{{Tags: map[string]string{"ALBUM": "x"}}})
	if err != nil {
		t.Fatalf("Consensus returned error: %v", err)
	}
	if got.Tags != nil || got.Metadata != nil {
		t.Fatalf("Consensus() = %+v, want empty domain.Fingerprint", got)
	}
}

func TestNoopFingerprinter_ContentTypesReturnsNil(t *testing.T) {
	f := service.NoopFingerprinter{}
	if got := f.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
