package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type NewProgressParams struct {
	UserID           uuid.UUID
	MaterialID       uuid.UUID
	Track            ProgressTrack
	PlanID           *uuid.UUID
	AlgorithmKey     string
	AlgorithmVersion int
	HorizonDays      int
	Now              time.Time
}

// NewProgress starts the first stage immediately. A late-added material gets
// the entire configured horizon starting now, independently of older materials.
func NewProgress(params NewProgressParams) (UserMaterialProgress, error) {
	if params.Now.IsZero() || params.HorizonDays < 0 || params.HorizonDays > 365 {
		return UserMaterialProgress{}, fmt.Errorf("invalid start or learning horizon")
	}
	if params.Track == ProgressTrackCram && (params.HorizonDays < 1 || params.HorizonDays > 7) {
		return UserMaterialProgress{}, fmt.Errorf("CRAM horizon must be between 1 and 7 days")
	}
	if params.Track == ProgressTrackLongTerm && params.HorizonDays < 7 {
		return UserMaterialProgress{}, fmt.Errorf("long-term horizon must be between 7 and 365 days")
	}
	now := params.Now.UTC()
	p := UserMaterialProgress{
		UserID: params.UserID, MaterialID: params.MaterialID, Track: params.Track,
		AlgorithmKey: params.AlgorithmKey, AlgorithmVersion: params.AlgorithmVersion,
		Stage: 1, Version: 1, LearningStartedAt: now, StageReviewAt: &now,
		CreatedAt: now, UpdatedAt: now,
	}
	if params.PlanID != nil {
		planID := *params.PlanID
		p.PlanID = &planID
	}
	if params.HorizonDays > 0 {
		target := now.AddDate(0, 0, params.HorizonDays)
		p.TargetAt = &target
	}
	if err := p.Validate(); err != nil {
		return UserMaterialProgress{}, err
	}
	return p, nil
}
