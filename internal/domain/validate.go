package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is the shared reflection-based validator instance for every
// kernel type's Validate() method. Struct tags express each type's
// invariants declaratively (see person.go, tag.go, etc.) instead of
// hand-written if-chains, so the rule and the field it applies to can never
// drift apart.
var validate = validator.New(validator.WithRequiredStructEnabled())

// FieldError describes one failed validation rule on one field, independent
// of the validation library that produced it. This is the shape meant to
// survive up to an API error response later (e.g. a gRPC BadRequest field
// violation or a REST 400 with per-field detail) — callers of Validate()
// should never need to know a reflection-based validator was involved.
type FieldError struct {
	// Field is the struct field name that failed (e.g. "Name").
	Field string
	// Rule is the violated rule (e.g. "required", "oneof").
	Rule string
	// Value is the invalid value, stringified for display.
	Value string
}

func (e FieldError) String() string {
	return fmt.Sprintf("%s: failed %q (got %q)", e.Field, e.Rule, e.Value)
}

// ValidationError collects every FieldError produced by one Validate call.
// It always holds at least one FieldError.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fe.String()
	}
	return strings.Join(parts, "; ")
}

// validateStruct runs struct-tag validation via reflection and translates
// the result into a *ValidationError, so domain callers and, eventually,
// API handlers only ever deal with FieldError — never the underlying
// validation library's own error type.
func validateStruct(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		// Not a validation failure (e.g. a malformed struct tag) — a
		// programmer error, not a domain validation failure, so it's
		// surfaced as-is rather than disguised as a ValidationError.
		return err
	}

	fieldErrs := make([]FieldError, 0, len(verrs))
	for _, fe := range verrs {
		fieldErrs = append(fieldErrs, FieldError{
			Field: fe.Field(),
			Rule:  fe.Tag(),
			Value: fmt.Sprintf("%v", fe.Value()),
		})
	}
	return &ValidationError{Errors: fieldErrs}
}
