package service_test

import (
	"context"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakeConfidenceScorer is a minimal ports.ConfidenceScorer double for
// exercising ConfidenceScoreRegistry's dispatch — every call returns a
// fixed, recognizable candidate list so tests can tell it apart from
// service.NoopConfidenceScorer's pass-through behavior.
type fakeConfidenceScorer struct {
	contentTypes []domain.ContentType
	scored       []domain.MatchCandidate
}

func (f *fakeConfidenceScorer) ContentTypes() []domain.ContentType { return f.contentTypes }

func (f *fakeConfidenceScorer) ConfidenceScore(_ context.Context, _ domain.Fingerprint, _ []domain.MatchCandidate) ([]domain.MatchCandidate, error) {
	return f.scored, nil
}

func TestConfidenceScoreRegistry_DispatchesToRegisteredImplementation(t *testing.T) {
	music := &fakeConfidenceScorer{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		scored:       []domain.MatchCandidate{{ExternalRef: "fixed-ref", Score: 0.9}},
	}
	registry := service.NewConfidenceScoreRegistry(music)

	got, err := registry.ConfidenceScore(context.Background(), domain.ContentTypeMusic, domain.Fingerprint{}, nil)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 1 || got[0].ExternalRef != "fixed-ref" || got[0].Score != 0.9 {
		t.Fatalf("ConfidenceScore() = %+v, want one candidate with ExternalRef=fixed-ref Score=0.9", got)
	}
}

func TestConfidenceScoreRegistry_FallsBackToNoopWhenUnregistered(t *testing.T) {
	music := &fakeConfidenceScorer{
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		scored:       []domain.MatchCandidate{{ExternalRef: "fixed-ref", Score: 0.9}},
	}
	registry := service.NewConfidenceScoreRegistry(music)

	candidates := []domain.MatchCandidate{{ExternalRef: "untouched"}}
	got, err := registry.ConfidenceScore(context.Background(), domain.ContentTypeMovie, domain.Fingerprint{}, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 1 || got[0].ExternalRef != "untouched" || got[0].Score != 0 {
		t.Fatalf("ConfidenceScore() = %+v, want candidates unchanged (NoopConfidenceScorer fallback)", got)
	}
}

func TestConfidenceScoreRegistry_NoRegistrationsFallsBackForEveryContentType(t *testing.T) {
	registry := service.NewConfidenceScoreRegistry()

	candidates := []domain.MatchCandidate{{ExternalRef: "untouched"}}
	got, err := registry.ConfidenceScore(context.Background(), domain.ContentTypeMusic, domain.Fingerprint{}, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 1 || got[0].ExternalRef != "untouched" {
		t.Fatalf("ConfidenceScore() = %+v, want candidates unchanged", got)
	}
}

func TestNoopConfidenceScorer_ReturnsCandidatesUnchanged(t *testing.T) {
	s := service.NoopConfidenceScorer{}

	candidates := []domain.MatchCandidate{{ExternalRef: "untouched"}}
	got, err := s.ConfidenceScore(context.Background(), domain.Fingerprint{}, candidates)
	if err != nil {
		t.Fatalf("ConfidenceScore returned error: %v", err)
	}
	if len(got) != 1 || got[0].ExternalRef != "untouched" {
		t.Fatalf("ConfidenceScore() = %+v, want candidates unchanged", got)
	}
}

func TestNoopConfidenceScorer_ContentTypesReturnsNil(t *testing.T) {
	s := service.NoopConfidenceScorer{}
	if got := s.ContentTypes(); got != nil {
		t.Fatalf("ContentTypes() = %v, want nil", got)
	}
}
