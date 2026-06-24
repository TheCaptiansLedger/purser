package domain_test

import (
	"errors"
	"purser/internal/domain"
	"testing"
)

func TestValidateTransition(t *testing.T) {
	cases := []struct {
		from    domain.ItemStatus
		to      domain.ItemStatus
		wantErr bool
	}{
		// wanted → allowed
		{domain.StatusWanted, domain.StatusWanted, false},
		{domain.StatusWanted, domain.StatusSkipped, false},
		// wanted → forbidden
		{domain.StatusWanted, domain.StatusGrabbed, true},
		{domain.StatusWanted, domain.StatusDownloading, true},
		{domain.StatusWanted, domain.StatusImported, true},
		{domain.StatusWanted, domain.StatusMissing, true},
		// grabbed → all forbidden (pipeline-locked)
		{domain.StatusGrabbed, domain.StatusWanted, true},
		{domain.StatusGrabbed, domain.StatusSkipped, true},
		{domain.StatusGrabbed, domain.StatusDownloading, true},
		{domain.StatusGrabbed, domain.StatusImported, true},
		// downloading → all forbidden (pipeline-locked)
		{domain.StatusDownloading, domain.StatusWanted, true},
		{domain.StatusDownloading, domain.StatusSkipped, true},
		// imported → allowed
		{domain.StatusImported, domain.StatusWanted, false},
		// imported → forbidden
		{domain.StatusImported, domain.StatusSkipped, true},
		{domain.StatusImported, domain.StatusGrabbed, true},
		// missing → allowed
		{domain.StatusMissing, domain.StatusWanted, false},
		{domain.StatusMissing, domain.StatusSkipped, false},
		// missing → forbidden
		{domain.StatusMissing, domain.StatusGrabbed, true},
		{domain.StatusMissing, domain.StatusDownloading, true},
		// skipped → allowed
		{domain.StatusSkipped, domain.StatusWanted, false},
		{domain.StatusSkipped, domain.StatusSkipped, false},
		// skipped → forbidden
		{domain.StatusSkipped, domain.StatusGrabbed, true},
		{domain.StatusSkipped, domain.StatusDownloading, true},
		{domain.StatusSkipped, domain.StatusImported, true},
	}

	for _, tc := range cases {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			err := domain.ValidateTransition(tc.from, tc.to)
			if tc.wantErr && err == nil {
				t.Errorf("domain.ValidateTransition(%q, %q) = nil, want error", tc.from, tc.to)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("domain.ValidateTransition(%q, %q) = %v, want nil", tc.from, tc.to, err)
			}
			if tc.wantErr && err != nil && !errors.Is(err, domain.ErrInvalidTransition) {
				t.Errorf("domain.ValidateTransition(%q, %q) error does not wrap ErrInvalidTransition: %v", tc.from, tc.to, err)
			}
		})
	}
}
