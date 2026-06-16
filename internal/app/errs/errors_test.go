package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrNotFound_IsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("IsNotFound(ErrNotFound) = false, want true")
	}
	if IsNotFound(errors.New("other")) {
		t.Error("IsNotFound(other error) = true, want false")
	}
	wrapped := fmt.Errorf("wrap: %w", ErrNotFound)
	if !IsNotFound(wrapped) {
		t.Error("IsNotFound(wrapped ErrNotFound) = false, want true")
	}
}

func TestValidation(t *testing.T) {
	err := Validation("name is required")
	if err == nil {
		t.Fatal("Validation() returned nil")
	}
	if err.Error() != "name is required" {
		t.Errorf("Error() = %q, want %q", err.Error(), "name is required")
	}
	if !IsValidation(err) {
		t.Error("IsValidation(validation error) = false, want true")
	}
	if IsValidation(ErrNotFound) {
		t.Error("IsValidation(ErrNotFound) = true, want false")
	}
	if IsValidation(errors.New("plain")) {
		t.Error("IsValidation(plain error) = true, want false")
	}
}
