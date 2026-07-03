package domain

import (
	"errors"
	"fmt"
)

// ErrInvalidTransition is returned when a user-initiated status change is not
// permitted from the item's current state.
var ErrInvalidTransition = errors.New("invalid status transition")

// ItemStatus tracks the acquisition state of a leaf item.
type ItemStatus string

// Item status constants covering the full acquisition state machine.
const (
	StatusWanted      ItemStatus = "wanted"
	StatusGrabbed     ItemStatus = "grabbed"
	StatusDownloading ItemStatus = "downloading"
	StatusImported    ItemStatus = "imported"
	StatusMissing     ItemStatus = "missing"
	StatusSkipped     ItemStatus = "skipped"
)

// legalUserTransitions maps each status to the set of statuses a user may
// set via PATCH. Grabbed and downloading are locked by the acquisition pipeline
// and cannot be changed by user action.
var legalUserTransitions = map[ItemStatus]map[ItemStatus]bool{
	StatusWanted:      {StatusWanted: true, StatusSkipped: true},
	StatusGrabbed:     {},
	StatusDownloading: {},
	StatusImported:    {StatusWanted: true},
	StatusMissing:     {StatusWanted: true, StatusSkipped: true},
	StatusSkipped:     {StatusWanted: true, StatusSkipped: true},
}

// ValidateTransition checks whether a user-initiated status change is legal.
// It is NOT used by the scan pipeline. The scan service writes item status
// directly because file presence on disk is ground truth: a matched file is
// always imported regardless of the item's prior status.
func ValidateTransition(from, to ItemStatus) error {
	allowed, ok := legalUserTransitions[from]
	if !ok || !allowed[to] {
		return fmt.Errorf("cannot transition item from %q to %q: %w", from, to, ErrInvalidTransition)
	}
	return nil
}
