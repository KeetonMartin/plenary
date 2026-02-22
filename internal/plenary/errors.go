package plenary

import "errors"

var (
	ErrValidation = errors.New("validation_error")
	ErrConflict   = errors.New("conflict_error")
	ErrNotFound   = errors.New("not_found")
)

