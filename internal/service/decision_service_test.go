package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/service"
	"testing"
)

// fakePersisterResolver is a minimal ports.PersisterResolver double for
// exercising DecisionService in isolation — it never imports anything
// content-type-specific, matching #517's requirement that DecisionService
// be unit-testable against a fake Persister with zero Music knowledge.
type fakePersisterResolver struct {
	err error

	called      bool
	contentType domain.ContentType
	fingerprint *domain.Fingerprint
	candidate   domain.MatchCandidate
	files       []*domain.UnmatchedFile
}

func (f *fakePersisterResolver) Persist(_ context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidate domain.MatchCandidate, files []*domain.UnmatchedFile) error {
	f.called = true
	f.contentType = contentType
	f.fingerprint = fingerprint
	f.candidate = candidate
	f.files = files
	return f.err
}

func TestDecisionService_Decide_ThresholdBoundary(t *testing.T) {
	tests := []struct {
		name          string
		score         float64
		wantPersisted bool
	}{
		{name: "just below threshold stays in review queue", score: 0.74, wantPersisted: false},
		{name: "exactly at threshold clears it", score: 0.75, wantPersisted: true},
		{name: "just above threshold clears it", score: 0.76, wantPersisted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &fakePersisterResolver{}
			svc := service.NewDecisionService(0.75, resolver)
			candidates := []domain.MatchCandidate{{ExternalRef: "candidate", Score: tt.score}}

			persisted, err := svc.Decide(context.Background(), domain.ContentTypeMusic, &domain.Fingerprint{}, candidates, nil)
			if err != nil {
				t.Fatalf("Decide returned error: %v", err)
			}
			if persisted != tt.wantPersisted {
				t.Fatalf("Decide() persisted = %v, want %v", persisted, tt.wantPersisted)
			}
			if resolver.called != tt.wantPersisted {
				t.Fatalf("Persist called = %v, want %v", resolver.called, tt.wantPersisted)
			}
		})
	}
}

func TestDecisionService_Decide_EmptyCandidatesLeftAlone(t *testing.T) {
	resolver := &fakePersisterResolver{}
	svc := service.NewDecisionService(0.75, resolver)

	persisted, err := svc.Decide(context.Background(), domain.ContentTypeMusic, &domain.Fingerprint{}, nil, nil)
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if persisted {
		t.Fatal("Decide() persisted = true, want false for an empty candidates list")
	}
	if resolver.called {
		t.Fatal("Decide dispatched to Persist for an empty candidates list")
	}
}

func TestDecisionService_Decide_PicksTopScoredCandidateRegardlessOfOrder(t *testing.T) {
	resolver := &fakePersisterResolver{}
	svc := service.NewDecisionService(0.75, resolver)
	candidates := []domain.MatchCandidate{
		{ExternalRef: "runner-up", Score: 0.8},
		{ExternalRef: "winner", Score: 0.95},
		{ExternalRef: "also-ran", Score: 0.5},
	}

	persisted, err := svc.Decide(context.Background(), domain.ContentTypeMusic, &domain.Fingerprint{}, candidates, nil)
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if !persisted {
		t.Fatal("Decide() persisted = false, want true")
	}
	if resolver.candidate.ExternalRef != "winner" {
		t.Fatalf("Persist called with candidate %+v, want ExternalRef=winner", resolver.candidate)
	}
}

func TestDecisionService_Decide_PropagatesPersistError(t *testing.T) {
	wantErr := errors.New("persist failed")
	resolver := &fakePersisterResolver{err: wantErr}
	svc := service.NewDecisionService(0.75, resolver)
	candidates := []domain.MatchCandidate{{ExternalRef: "candidate", Score: 0.9}}

	persisted, err := svc.Decide(context.Background(), domain.ContentTypeMusic, &domain.Fingerprint{}, candidates, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Decide returned error %v, want %v", err, wantErr)
	}
	if persisted {
		t.Fatal("Decide() persisted = true, want false when Persist fails")
	}
}
