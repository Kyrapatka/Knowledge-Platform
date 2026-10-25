package model

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/google/uuid"
	"time"
)

type MockPreset struct {
	ID                   uuid.UUID     `json:"id" gorm:"type:uuid;primaryKey"`
	UserID               uuid.UUID     `json:"-" gorm:"type:uuid"`
	Name                 string        `json:"name"`
	FolderIDs            []uuid.UUID   `json:"folder_ids" gorm:"serializer:json;type:jsonb"`
	Settings             *graph.Config `json:"settings" gorm:"serializer:json;type:jsonb"`
	Version              int           `json:"version"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
	LastUsedAt           *time.Time    `json:"last_used_at"`
	UnavailableFolderIDs []uuid.UUID   `json:"unavailable_folder_ids" gorm:"-"`
}
