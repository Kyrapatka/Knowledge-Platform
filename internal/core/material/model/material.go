package model

import (
	"time"

	"github.com/google/uuid"
)

type Material struct {
	ID       uuid.UUID
	FolderID uuid.UUID

	Values     map[string]*string
	Metadata   map[string]*string
	Difficulty Difficulty

	CreatedAt time.Time
	UpdatedAt time.Time
}
