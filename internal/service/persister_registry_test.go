package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakePersister is a minimal ports.Persister double for exercising
// PersisterRegistry's dispatch — records the args it was called with and
// returns a fixed error so tests can tell it apart from
// service.NoopPersister's do-nothing behavior.
type fakePersister struct {
	contentTypes []domain.ContentType
	err          error

	called    bool
	candidate domain.MatchCandidate
}

func (f *fakePersister) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakePersister) Persist(_ context.Context, _ *domain.Fingerprint, candidate domain.MatchCandidate, _ []*domain.UnmatchedFile) error {
	f.called = true
	f.candidate = candidate
	return f.err
}

func TestPersisterRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakePersister{contentTypes: []domain.ContentType{domain.ContentTypeMusic}}
	registry := service.NewPersisterRegistry(music)

	candidate := domain.MatchCandidate{ExternalRef: "release-mbid"}
	if err := registry.Persist(context.Background(), domain.ContentTypeMusic, nil, candidate, nil); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}
	if !music.called {
		t.Fatal("Persist did not dispatch to the registered fakePersister")
	}
	if music.candidate.ExternalRef != "release-mbid" {
		t.Fatalf("Persist called with candidate %+v, want ExternalRef=release-mbid", music.candidate)
	}
}

func TestPersisterRegistry_PropagatesPersisterError(t *testing.T) {
	wantErr := errors.New("persist failed")
	music := &fakePersister{contentTypes: []domain.ContentType{domain.ContentTypeMusic}, err: wantErr}
	registry := service.NewPersisterRegistry(music)

	err := registry.Persist(context.Background(), domain.ContentTypeMusic, nil, domain.MatchCandidate{}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Persist returned error %v, want %v", err, wantErr)
	}
}

func TestPersisterRegistry_FallsBackToNoopWhenUnregistered(t *testing.T) {
	music := &fakePersister{contentTypes: []domain.ContentType{domain.ContentTypeMusic}}
	registry := service.NewPersisterRegistry(music)

	if err := registry.Persist(context.Background(), domain.ContentTypeMovie, nil, domain.MatchCandidate{}, nil); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}
	if music.called {
		t.Fatal("Persist dispatched to the music fakePersister for an unregistered content type")
	}
}

func TestPersisterRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewPersisterRegistry()

	if err := registry.Persist(context.Background(), domain.ContentTypeMusic, nil, domain.MatchCandidate{}, nil); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}
}

func TestNoopPersister_ReturnsNil(t *testing.T) {
	p := service.NoopPersister{}
	if err := p.Persist(context.Background(), nil, domain.MatchCandidate{}, nil); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}
}

func TestNoopPersister_ContentTypesReturnsNil(t *testing.T) {
	p := service.NoopPersister{}
	if got := p.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
