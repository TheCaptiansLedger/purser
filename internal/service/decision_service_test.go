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
	candidates  []domain.MatchCandidate
	files       []*domain.UnmatchedFile
}

func (f *fakePersisterResolver) Persist(_ context.Context, contentType domain.ContentType, fingerprint *domain.Fingerprint, candidates []domain.MatchCandidate, files []*domain.UnmatchedFile) error {
	f.called = true
	f.contentType = contentType
	f.fingerprint = fingerprint
	f.candidates = candidates
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

// TestDecisionService_Decide_CollectsEveryCandidateAtOrAboveThreshold covers
// the N-candidates case: every provider (e.g. StashDB and ThePornDB scored
// independently, per ADR-0027) that clears threshold is collected and
// dispatched together, in input order — DecisionService never picks a
// single winner or re-sorts by Score.
func TestDecisionService_Decide_CollectsEveryCandidateAtOrAboveThreshold(t *testing.T) {
	resolver := &fakePersisterResolver{}
	svc := service.NewDecisionService(0.75, resolver)
	candidates := []domain.MatchCandidate{
		{ExternalRef: "stashdb", Score: 0.8},
		{ExternalRef: "below-threshold", Score: 0.5},
		{ExternalRef: "theporndb", Score: 0.95},
	}

	persisted, err := svc.Decide(context.Background(), domain.ContentTypeMusic, &domain.Fingerprint{}, candidates, nil)
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if !persisted {
		t.Fatal("Decide() persisted = false, want true")
	}
	want := []string{"stashdb", "theporndb"}
	if len(resolver.candidates) != len(want) {
		t.Fatalf("Persist called with %d candidates, want %d: %+v", len(resolver.candidates), len(want), resolver.candidates)
	}
	for i, ref := range want {
		if resolver.candidates[i].ExternalRef != ref {
			t.Fatalf("Persist candidates[%d].ExternalRef = %q, want %q (input order preserved, no ranking)", i, resolver.candidates[i].ExternalRef, ref)
		}
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
