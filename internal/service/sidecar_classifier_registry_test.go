package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeSidecarClassifier is a minimal ports.SidecarClassifier double for
// exercising SidecarClassifierRegistry's dispatch — every registered path
// classifies as a fixed SidecarKind regardless of input, so tests can tell
// it apart from service.NoopClassifier's always-None behavior.
type fakeSidecarClassifier struct {
	contentTypes []domain.ContentType
	kind         ports.SidecarKind
	err          error
}

func (f *fakeSidecarClassifier) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeSidecarClassifier) Classify(_ context.Context, _ string) (ports.SidecarKind, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.kind, nil
}

func TestSidecarClassifierRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakeSidecarClassifier{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, kind: ports.SidecarKindImage}
	registry := service.NewSidecarClassifierRegistry(music)

	got, err := registry.Classify(context.Background(), domain.ContentTypeMusic, "/media/cover.jpg")
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if got != ports.SidecarKindImage {
		t.Fatalf("Classify() = %q, want %q", got, ports.SidecarKindImage)
	}
}

func TestSidecarClassifierRegistry_FallsBackToNoopClassifierWhenUnregistered(t *testing.T) {
	music := &fakeSidecarClassifier{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, kind: ports.SidecarKindOther}
	registry := service.NewSidecarClassifierRegistry(music)

	got, err := registry.Classify(context.Background(), domain.ContentTypeMovie, "/media/movie.mkv")
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if got != ports.SidecarKindNone {
		t.Fatalf("Classify() = %q, want %q (NoopClassifier fallback)", got, ports.SidecarKindNone)
	}
}

func TestSidecarClassifierRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewSidecarClassifierRegistry()

	got, err := registry.Classify(context.Background(), domain.ContentTypeMusic, "/media/a.flac")
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if got != ports.SidecarKindNone {
		t.Fatalf("Classify() = %q, want %q", got, ports.SidecarKindNone)
	}
}

func TestSidecarClassifierRegistry_PropagatesImplementationError(t *testing.T) {
	wantErr := errors.New("boom")
	music := &fakeSidecarClassifier{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, err: wantErr}
	registry := service.NewSidecarClassifierRegistry(music)

	_, err := registry.Classify(context.Background(), domain.ContentTypeMusic, "/media/a.flac")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Classify returned %v, want wrapping %v", err, wantErr)
	}
}

func TestNoopClassifier_AlwaysClassifiesAsNone(t *testing.T) {
	c := service.NoopClassifier{}

	got, err := c.Classify(context.Background(), "/media/a.flac")
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if got != ports.SidecarKindNone {
		t.Fatalf("Classify() = %q, want %q", got, ports.SidecarKindNone)
	}
}

func TestNoopClassifier_ContentTypesReturnsNil(t *testing.T) {
	c := service.NoopClassifier{}
	if got := c.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
