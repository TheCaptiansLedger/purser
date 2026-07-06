package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"testing"
)

func runMusicScanGroupContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()
	if s.MusicScanGroups == nil {
		t.Skip("not implemented yet")
	}

	t.Run("SaveAndGet", func(t *testing.T) {
		ctx := context.Background()
		g := &domain.MusicScanGroup{
			FolderPath:  "/music/test/album-saveandget",
			TotalTracks: 10,
			TotalDiscs:  1,
			Status:      domain.UnmatchedPending,
		}
		if err := s.MusicScanGroups.Save(ctx, g); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if g.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.MusicScanGroups.Get(ctx, g.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.FolderPath != g.FolderPath {
			t.Errorf("FolderPath = %q, want %q", got.FolderPath, g.FolderPath)
		}
		if got.TotalTracks != g.TotalTracks {
			t.Errorf("TotalTracks = %d, want %d", got.TotalTracks, g.TotalTracks)
		}
		if got.TotalDiscs != g.TotalDiscs {
			t.Errorf("TotalDiscs = %d, want %d", got.TotalDiscs, g.TotalDiscs)
		}
		if got.Status != g.Status {
			t.Errorf("Status = %q, want %q", got.Status, g.Status)
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		ctx := context.Background()
		pending := &domain.MusicScanGroup{
			FolderPath:  "/music/list-pending",
			TotalTracks: 5,
			Status:      domain.UnmatchedPending,
		}
		matched := &domain.MusicScanGroup{
			FolderPath:  "/music/list-matched",
			TotalTracks: 8,
			Status:      domain.UnmatchedMatched,
		}
		dismissed := &domain.MusicScanGroup{
			FolderPath:  "/music/list-dismissed",
			TotalTracks: 3,
			Status:      domain.UnmatchedDismissed,
		}
		for _, g := range []*domain.MusicScanGroup{pending, matched, dismissed} {
			if err := s.MusicScanGroups.Save(ctx, g); err != nil {
				t.Fatalf("Save %s: %v", g.Status, err)
			}
		}
		results, err := s.MusicScanGroups.List(ctx, domain.UnmatchedPending)
		if err != nil {
			t.Fatalf("List pending: %v", err)
		}
		var found bool
		for _, r := range results {
			if r.Status != domain.UnmatchedPending {
				t.Errorf("List(pending) returned entry with status %q", r.Status)
			}
			if r.ID == pending.ID {
				found = true
			}
		}
		if !found {
			t.Error("pending entry not found in List(pending) results")
		}
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		ctx := context.Background()
		g := &domain.MusicScanGroup{
			FolderPath:  "/music/update-status",
			TotalTracks: 4,
			Status:      domain.UnmatchedPending,
		}
		if err := s.MusicScanGroups.Save(ctx, g); err != nil {
			t.Fatalf("Save: %v", err)
		}
		g.Status = domain.UnmatchedMatched
		if err := s.MusicScanGroups.Save(ctx, g); err != nil {
			t.Fatalf("Save updated: %v", err)
		}
		got, err := s.MusicScanGroups.Get(ctx, g.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Status != domain.UnmatchedMatched {
			t.Errorf("Status = %q, want matched", got.Status)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx := context.Background()
		g := &domain.MusicScanGroup{
			FolderPath:  "/music/delete-me",
			TotalTracks: 6,
			Status:      domain.UnmatchedPending,
		}
		if err := s.MusicScanGroups.Save(ctx, g); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := s.MusicScanGroups.Delete(ctx, g.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err := s.MusicScanGroups.Get(ctx, g.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("CandidatesRoundTrip", func(t *testing.T) {
		ctx := context.Background()
		signals := domain.MusicConfidenceSignals{
			Barcode:       0.0,
			ISRC:          0.95,
			RGNameFuzzy:   0.62,
			TrackCount:    0.20,
			TrackTitleSet: 0.45,
			Duration:      0.88,
			AcoustID:      0.0,
		}
		candidate := domain.MusicReleaseCandidate{
			ArtistName:        "REO Speedwagon",
			ReleaseGroupTitle: "Hi Infidelity",
			ReleaseTitle:      "Hi Infidelity (2024 Remaster)",
			OverallConfidence: 0.72,
			Signals:           signals,
		}
		g := &domain.MusicScanGroup{
			FolderPath:  "/music/candidates-rt",
			TotalTracks: 10,
			Status:      domain.UnmatchedPending,
			Candidates:  []domain.MusicReleaseCandidate{candidate},
		}
		if err := s.MusicScanGroups.Save(ctx, g); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.MusicScanGroups.Get(ctx, g.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got.Candidates) != 1 {
			t.Fatalf("Candidates len = %d, want 1", len(got.Candidates))
		}
		gotSig := got.Candidates[0].Signals
		if gotSig.Barcode != signals.Barcode {
			t.Errorf("Signals.Barcode = %v, want %v", gotSig.Barcode, signals.Barcode)
		}
		if gotSig.ISRC != signals.ISRC {
			t.Errorf("Signals.ISRC = %v, want %v", gotSig.ISRC, signals.ISRC)
		}
		if gotSig.RGNameFuzzy != signals.RGNameFuzzy {
			t.Errorf("Signals.RGNameFuzzy = %v, want %v", gotSig.RGNameFuzzy, signals.RGNameFuzzy)
		}
		if gotSig.TrackCount != signals.TrackCount {
			t.Errorf("Signals.TrackCount = %v, want %v", gotSig.TrackCount, signals.TrackCount)
		}
		if gotSig.TrackTitleSet != signals.TrackTitleSet {
			t.Errorf("Signals.TrackTitleSet = %v, want %v", gotSig.TrackTitleSet, signals.TrackTitleSet)
		}
		if gotSig.Duration != signals.Duration {
			t.Errorf("Signals.Duration = %v, want %v", gotSig.Duration, signals.Duration)
		}
		if gotSig.AcoustID != signals.AcoustID {
			t.Errorf("Signals.AcoustID = %v, want %v", gotSig.AcoustID, signals.AcoustID)
		}
		if got.Candidates[0].ArtistName != candidate.ArtistName {
			t.Errorf("ArtistName = %q, want %q", got.Candidates[0].ArtistName, candidate.ArtistName)
		}
		if got.Candidates[0].OverallConfidence != candidate.OverallConfidence {
			t.Errorf("OverallConfidence = %v, want %v", got.Candidates[0].OverallConfidence, candidate.OverallConfidence)
		}
	})
}
