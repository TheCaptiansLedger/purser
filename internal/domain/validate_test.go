package domain

import (
	"errors"
	"testing"
)

func TestValidateStruct_Success(t *testing.T) {
	type s struct {
		Name string `validate:"required"`
	}
	if err := validateStruct(s{Name: "ok"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateStruct_Failure(t *testing.T) {
	type s struct {
		Name string `validate:"required"`
		Kind string `validate:"required,oneof=a b"`
	}

	err := validateStruct(s{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(verr.Errors) != 2 {
		t.Fatalf("expected 2 field errors, got %d: %v", len(verr.Errors), verr.Errors)
	}

	byField := map[string]FieldError{}
	for _, fe := range verr.Errors {
		byField[fe.Field] = fe
	}

	nameErr, ok := byField["Name"]
	if !ok {
		t.Fatal("expected a field error for Name")
	}
	if nameErr.Rule != "required" {
		t.Errorf("Name rule = %q, want %q", nameErr.Rule, "required")
	}

	kindErr, ok := byField["Kind"]
	if !ok {
		t.Fatal("expected a field error for Kind")
	}
	if kindErr.Rule != "required" {
		t.Errorf("Kind rule = %q, want %q", kindErr.Rule, "required")
	}
}

func TestValidateStruct_OneofFailure(t *testing.T) {
	type s struct {
		Kind string `validate:"required,oneof=a b"`
	}

	err := validateStruct(s{Kind: "c"})
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ValidationError, got %T (%v)", err, err)
	}
	if len(verr.Errors) != 1 {
		t.Fatalf("expected 1 field error, got %d: %v", len(verr.Errors), verr.Errors)
	}
	if verr.Errors[0].Rule != "oneof" {
		t.Errorf("Rule = %q, want %q", verr.Errors[0].Rule, "oneof")
	}
	if verr.Errors[0].Value != "c" {
		t.Errorf("Value = %q, want %q", verr.Errors[0].Value, "c")
	}
}

func TestValidateStruct_NonValidationError(t *testing.T) {
	// validate.Struct on a non-struct value produces a
	// *validator.InvalidValidationError, not validator.ValidationErrors —
	// validateStruct must surface that as-is rather than disguising a
	// programmer error as a domain ValidationError.
	err := validateStruct("not a struct")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var verr *ValidationError
	if errors.As(err, &verr) {
		t.Fatalf("expected a non-ValidationError, got %v", verr)
	}
}

func TestFieldError_String(t *testing.T) {
	fe := FieldError{Field: "Name", Rule: "required", Value: ""}
	got := fe.String()
	want := `Name: failed "required" (got "")`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestValidationError_Error(t *testing.T) {
	verr := &ValidationError{Errors: []FieldError{
		{Field: "Name", Rule: "required", Value: ""},
		{Field: "Kind", Rule: "required", Value: ""},
	}}
	got := verr.Error()
	want := `Name: failed "required" (got ""); Kind: failed "required" (got "")`
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
