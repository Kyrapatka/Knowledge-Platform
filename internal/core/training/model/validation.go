package model

import (
	"fmt"
	"github.com/google/uuid"
)

func (p UserMaterialProgress) Validate() error {
	if p.UserID == uuid.Nil || p.MaterialID == uuid.Nil {
		return fmt.Errorf("progress identity is required")
	}
	if p.Track != ProgressTrackDefault && p.Track != ProgressTrackLongTerm && p.Track != ProgressTrackCram {
		return fmt.Errorf("invalid track")
	}
	if (p.Track == ProgressTrackCram) != (p.PlanID != nil) || (p.PlanID != nil && *p.PlanID == uuid.Nil) {
		return fmt.Errorf("only CRAM progress must have a plan ID")
	}
	if p.AlgorithmKey == "" || p.AlgorithmVersion < 1 || p.Stage < 1 || p.Version < 1 {
		return fmt.Errorf("invalid algorithm, stage or version")
	}
	if p.CorrectCount < 0 || p.WrongCount < 0 || p.ConsecutiveCorrect < 0 || p.ConsecutiveWrong < 0 || p.RehabConsecutiveCorrect < 0 {
		return fmt.Errorf("negative answer counter")
	}
	if p.DifficultyOverride != nil && !p.DifficultyOverride.Valid() {
		return fmt.Errorf("invalid difficulty override")
	}
	if p.LearningStartedAt.IsZero() || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return fmt.Errorf("progress timestamps are required")
	}
	if p.TargetAt != nil && !p.TargetAt.After(p.LearningStartedAt) {
		return fmt.Errorf("target must follow material learning start")
	}
	if p.RehabActive {
		if p.RehabReviewAt == nil || (p.RehabStep != 1 && p.RehabStep != 2) || p.ExtraReviewAt != nil {
			return fmt.Errorf("invalid active rehab")
		}
	} else if p.RehabReviewAt != nil || p.RehabStep != 0 || p.RehabConsecutiveCorrect != 0 {
		return fmt.Errorf("inactive rehab must be cleared")
	}
	return nil
}
