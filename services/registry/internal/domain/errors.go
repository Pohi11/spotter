package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors. Adapters return these (wrapped is fine), and callers test with errors.Is.
var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrConflict           = errors.New("conflict: the version changed since it was read")
	ErrProductionOccupied = errors.New("the model already has a production version")
	ErrIllegalTransition  = errors.New("illegal stage transition")
	ErrInvalid            = errors.New("invalid input")
)

// ValidationError names the field that failed and why. errors.Is(err, ErrInvalid) is true.
type ValidationError struct {
	Field   string
	Problem string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Problem) }
func (e *ValidationError) Unwrap() error { return ErrInvalid }

// TransitionError names both stages of a rejected transition.
// errors.Is(err, ErrIllegalTransition) is true.
type TransitionError struct {
	From, To Stage
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("illegal stage transition from %s to %s", e.From, e.To)
}
func (e *TransitionError) Unwrap() error { return ErrIllegalTransition }

func invalid(field, problem string) error { return &ValidationError{Field: field, Problem: problem} }
