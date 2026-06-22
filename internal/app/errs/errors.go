package errs

import "errors"

// ErrNotFound is returned by services when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrLocked is returned by ConfigService.Set when the key is managed by the
// operator via env var or config file and cannot be overwritten from the UI.
var ErrLocked = errors.New("config key is locked by operator")

// ValidationError is returned when caller-supplied input fails validation.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Validation returns a ValidationError with the given message.
func Validation(msg string) error { return &ValidationError{Message: msg} }

// IsNotFound reports whether err represents a not-found condition.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsValidation reports whether err is a ValidationError.
func IsValidation(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}
