package material

import "errors"

var (
	ErrNotFound          = errors.New("material not found")
	ErrFolderNotFound    = errors.New("folder not found")
	ErrInvalidValues     = errors.New("invalid material values")
	ErrInvalidMetadata   = errors.New("invalid material metadata")
	ErrInvalidDifficulty = errors.New("invalid material difficulty")
)
