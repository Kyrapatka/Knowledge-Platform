package model

import (
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/google/uuid"
	"time"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
)

// PlanConfig is a resolved snapshot. HorizonDays is a duration applied afresh
// to each material, not a shared deadline measured from Plan.StartedAt.
type PlanConfig struct {
	Cards       map[string]folderconfig.FolderConfig `json:"cards"`
	PoolSize    int                                  `json:"pool_size"`
	HorizonDays int                                  `json:"horizon_days"`
}

type TrainingPlan struct {
	Version          int           `json:"version"`
	ID               uuid.UUID     `json:"id"`
	UserID           uuid.UUID     `json:"user_id"`
	Track            ProgressTrack `json:"track"`
	AlgorithmKey     string        `json:"algorithm_key"`
	AlgorithmVersion int           `json:"algorithm_version"`
	Status           Status        `json:"status"`
	Config           PlanConfig    `json:"config"`
	SourceFolderIDs  []uuid.UUID   `json:"source_folder_ids"`
	StartedAt        time.Time     `json:"started_at"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type TrainingSession struct {
	Combined   bool            `json:"combined"`
	Selection  []SessionSource `json:"selection" gorm:"serializer:json;type:jsonb"`
	ID         uuid.UUID       `json:"id"`
	PlanID     uuid.UUID       `json:"plan_id"`
	UserID     uuid.UUID       `json:"user_id"`
	Status     Status          `json:"status"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	CreatedAt  time.Time       `json:"created_at"`
}
