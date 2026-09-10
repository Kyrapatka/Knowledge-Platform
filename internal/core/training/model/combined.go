package model

import (
	"github.com/google/uuid"
	"time"
)

// SessionSource is an immutable launch selection. Empty Topics means all topics;
// __none__ selects materials without a topic. It never changes a material's metadata.
type SessionSource struct {
	FolderID uuid.UUID `json:"folder_id"`
	Topics   []string  `json:"topics,omitempty"`
}

type CombinedComponent struct {
	Session  TrainingSession `json:"session"`
	Plan     TrainingPlan    `json:"plan"`
	Summary  SessionSummary  `json:"summary"`
	PoolSize int             `json:"pool_size"`
}

type CombinedCurrent struct {
	SessionID    uuid.UUID     `json:"session_id"`
	PlanID       uuid.UUID     `json:"plan_id"`
	AlgorithmKey string        `json:"algorithm_key"`
	Presentation *Presentation `json:"presentation"`
}

type CombinedView struct {
	Sessions     []CombinedComponent `json:"sessions"`
	Current      *CombinedCurrent    `json:"current"`
	Summary      SessionSummary      `json:"summary"`
	NextReviewAt *time.Time          `json:"next_review_at"`
	EmptyReason  string              `json:"empty_reason,omitempty"`
}

type Availability struct {
	Total        int64
	Pending      int64
	NextReviewAt *time.Time
}
