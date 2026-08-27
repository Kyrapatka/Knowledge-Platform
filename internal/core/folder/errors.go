package folder

import "errors"

var (
	ErrNotFound        = errors.New("folder not found")
	ErrInvalidTemplate = errors.New("invalid template")
	ErrConfigConflict  = errors.New("folder config conflict")
)
