package workshop

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid workshop config")
	ErrConflict      = errors.New("workshop config conflict")
)
