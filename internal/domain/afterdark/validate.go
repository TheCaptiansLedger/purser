// Package afterdark holds the AfterDark module's own domain types — role-
// specific data that references the shared kernel (internal/domain) by ID
// rather than embedding it, per docs/technical/shared-domain-model.md
// Part 3's Profile pattern. This is the first module built on top of the
// shared kernel; nothing in internal/domain is edited to accommodate it.
package afterdark

import (
	"errors"
	"fmt"
	"purser/internal/domain"

	"github.com/go-playground/validator/v10"
)

// validate is a package-local validator instance, deliberately not
// shared with internal/domain/validate.go's unexported one — a module
// package must be addable with zero edits to the kernel, and reaching
// into an unexported kernel helper would violate that. It still produces
// domain.FieldError/domain.ValidationError, the same types
// internal/api/connect's error-mapping helper already recognizes, so
// AfterDark validation errors map to CodeInvalidArgument exactly like
// every kernel entity's.
var validate = validator.New(validator.WithRequiredStructEnabled())

func validateStruct(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}

	fieldErrs := make([]domain.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		fieldErrs = append(fieldErrs, domain.FieldError{
			Field: fe.Field(),
			Rule:  fe.Tag(),
			Value: fmt.Sprintf("%v", fe.Value()),
		})
	}
	return &domain.ValidationError{Errors: fieldErrs}
}
