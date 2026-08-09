package config_test

import (
	"context"
	"errors"
	"purser/internal/config"
	"sync"
	"testing"
)

func TestNewLive_SeedsSnapshotFromRepository(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	live, err := config.NewLive(context.Background(), "", repo)
	if err != nil {
		t.Fatalf("NewLive returned error: %v", err)
	}
	if got := live.Get().Pipeline.ConfidenceThreshold; got != 0.9 {
		t.Fatalf("Get().Pipeline.ConfidenceThreshold = %v, want 0.9", got)
	}

	st, ok := statusOf(live.Statuses(), "pipeline.confidence_threshold")
	if !ok {
		t.Fatal("Statuses missing pipeline.confidence_threshold")
	}
	if st.Source != config.SourceDB || st.Locked {
		t.Fatalf("status = %+v, want Source=db Locked=false", st)
	}
}

func TestNewLive_InitialFailurePropagatesAndReturnsNil(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `not valid json`,
	})

	live, err := config.NewLive(context.Background(), "", repo)
	if err == nil {
		t.Fatal("NewLive with invalid stored JSON did not return an error")
	}
	if live != nil {
		t.Fatalf("NewLive returned non-nil *Live alongside an error: %+v", live)
	}
}

func TestLive_RefreshAppliesNewDBValue(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{})

	live, err := config.NewLive(context.Background(), "", repo)
	if err != nil {
		t.Fatalf("NewLive returned error: %v", err)
	}
	if got := live.Get().Pipeline.ConfidenceThreshold; got != 0.75 {
		t.Fatalf("Get().Pipeline.ConfidenceThreshold = %v, want default 0.75", got)
	}

	repo.data["pipeline.confidence_threshold"] = `0.9`
	if err := live.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if got := live.Get().Pipeline.ConfidenceThreshold; got != 0.9 {
		t.Fatalf("Get().Pipeline.ConfidenceThreshold after Refresh = %v, want 0.9", got)
	}
}

func TestLive_RefreshFallsBackToDefaultAfterDBValueDeleted(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	live, err := config.NewLive(context.Background(), "", repo)
	if err != nil {
		t.Fatalf("NewLive returned error: %v", err)
	}
	if got := live.Get().Pipeline.ConfidenceThreshold; got != 0.9 {
		t.Fatalf("Get().Pipeline.ConfidenceThreshold = %v, want 0.9", got)
	}

	// Simulate a ResetSetting: the stored value disappears entirely.
	delete(repo.data, "pipeline.confidence_threshold")
	if err := live.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if got := live.Get().Pipeline.ConfidenceThreshold; got != 0.75 {
		t.Fatalf("Get().Pipeline.ConfidenceThreshold after reset = %v, want default 0.75 (must not stick at the deleted DB value)", got)
	}

	st, ok := statusOf(live.Statuses(), "pipeline.confidence_threshold")
	if !ok {
		t.Fatal("Statuses missing pipeline.confidence_threshold")
	}
	if st.Locked || st.Source != config.SourceDefault {
		t.Fatalf("status after reset = %+v, want Locked=false Source=default", st)
	}
}

func TestLive_RefreshFailurePreservesPreviousSnapshot(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	live, err := config.NewLive(context.Background(), "", repo)
	if err != nil {
		t.Fatalf("NewLive returned error: %v", err)
	}

	repo.data["pipeline.confidence_threshold"] = `not valid json`
	if err := live.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh with invalid stored JSON did not return an error")
	}

	if got := live.Get().Pipeline.ConfidenceThreshold; got != 0.9 {
		t.Fatalf("Get().Pipeline.ConfidenceThreshold after failed Refresh = %v, want unchanged 0.9", got)
	}
}

func TestLive_RefreshPropagatesSettingsRepositoryListError(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{})

	live, err := config.NewLive(context.Background(), "", repo)
	if err != nil {
		t.Fatalf("NewLive returned error: %v", err)
	}

	repo.listErr = errors.New("datastore unavailable")
	if err := live.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh with a failing SettingsRepository.List did not return an error")
	}
}

// TestLive_ConcurrentGetAndRefresh exercises Live under the race detector:
// Get must never observe a torn/partial snapshot while Refresh is
// concurrently swapping one in.
func TestLive_ConcurrentGetAndRefresh(t *testing.T) {
	repo := newFakeSettingsRepository(map[string]string{
		"pipeline.confidence_threshold": `0.9`,
	})

	live, err := config.NewLive(context.Background(), "", repo)
	if err != nil {
		t.Fatalf("NewLive returned error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = live.Get()
			_ = live.Statuses()
		}()
		go func() {
			defer wg.Done()
			_ = live.Refresh(context.Background())
		}()
	}
	wg.Wait()
}
