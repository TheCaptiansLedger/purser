package service_test

import (
	"context"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakeIdentifier is a minimal ports.Identifier double for exercising
// IdentifierRegistry's dispatch — every call returns a fixed, recognizable
// candidate list so tests can tell it apart from service.NoopIdentifier's
// always-nil behavior.
type fakeIdentifier struct {
	contentTypes []domain.ContentType
	candidates   []domain.MatchCandidate
}

func (f *fakeIdentifier) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeIdentifier) Identify(_ context.Context, _ domain.Fingerprint, _ []string, _, _ string) ([]domain.MatchCandidate, error) {
	return f.candidates, nil
}

func TestIdentifierRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakeIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{ExternalRef: "fixed-ref"}},
	}
	registry := service.NewIdentifierRegistry(music)

	got, err := registry.Identify(context.Background(), domain.ContentTypeMusic, domain.Fingerprint{}, nil, "/music/Artist/Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 1 || got[0].ExternalRef != "fixed-ref" {
		t.Fatalf("Identify() = %+v, want one candidate with ExternalRef=fixed-ref", got)
	}
}

func TestIdentifierRegistry_FallsBackToNoopWhenUnregistered(t *testing.T) {
	music := &fakeIdentifier{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		candidates:   []domain.MatchCandidate{{ExternalRef: "fixed-ref"}},
	}
	registry := service.NewIdentifierRegistry(music)

	got, err := registry.Identify(context.Background(), domain.ContentTypeMovie, domain.Fingerprint{}, nil, "/movies/Some Movie (2020)", "/movies")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Identify() = %+v, want empty (NoopIdentifier fallback)", got)
	}
}

func TestIdentifierRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewIdentifierRegistry()

	got, err := registry.Identify(context.Background(), domain.ContentTypeMusic, domain.Fingerprint{}, nil, "/music/Artist/Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Identify() = %+v, want empty", got)
	}
}

func TestNoopIdentifier_AlwaysReturnsNoCandidates(t *testing.T) {
	i := service.NoopIdentifier{}

	got, err := i.Identify(context.Background(), domain.Fingerprint{}, nil, "/music/Artist/Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Identify() = %+v, want empty", got)
	}
}

func TestNoopIdentifier_ContentTypesReturnsNil(t *testing.T) {
	i := service.NoopIdentifier{}
	if got := i.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
