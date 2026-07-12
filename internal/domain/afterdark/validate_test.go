package afterdark

import (
	"errors"
	"purser/internal/domain"
	"testing"
)

func TestValidateStruct_NonValidationError(t *testing.T) {
	// validate.Struct on a non-struct value produces a
	// *validator.InvalidValidationError, not validator.ValidationErrors —
	// validateStruct must surface that as-is rather than disguising a
	// programmer error as a domain.ValidationError. Mirrors
	// internal/domain/validate_test.go's equivalent case.
	err := validateStruct("not a struct")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var verr *domain.ValidationError
	if errors.As(err, &verr) {
		t.Fatalf("expected a non-ValidationError, got %v", verr)
	}
}
