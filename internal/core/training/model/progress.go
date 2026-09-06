package model

import (
	"time"

	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/google/uuid"
)

type ProgressTrack string

const (
	ProgressTrackDefault  ProgressTrack = "default"
	ProgressTrackCram     ProgressTrack = "cram"
	ProgressTrackLongTerm ProgressTrack = "long_term"
)

// UserMaterialProgress outlives sessions and, for persistent tracks, plans.
type UserMaterialProgress struct {
	UserID     uuid.UUID
	MaterialID uuid.UUID
	Track      ProgressTrack

	// Set only for plan-scoped progress (CRAM). Persistent tracks use nil.
	PlanID *uuid.UUID

	AlgorithmKey     string
	AlgorithmVersion int
	Stage            int

	CorrectCount       int
	WrongCount         int
	ConsecutiveCorrect int
	ConsecutiveWrong   int
	DifficultyOverride *materialmodel.Difficulty

	// The learning horizon starts separately for each material, including those
	// added to a source folder after a plan has already started.
	LearningStartedAt time.Time
	TargetAt          *time.Time
	CompletedAt       *time.Time

	// StageLastReviewAt anchors the main schedule, including when an algorithm
	// change recalculates its interval. Rehab never changes either Stage date.
	StageLastReviewAt *time.Time
	StageReviewAt     *time.Time

	RehabActive             bool
	RehabReviewAt           *time.Time
	RehabStep               int
	RehabConsecutiveCorrect int

	// A follow-up ten days after the second rehab learning day. This is not
	// another rehab step and must not advance Stage mastery.
	ExtraReviewAt *time.Time

	// Version is reserved for optimistic locking by the persistence layer.
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NextReviewAt returns the earliest active review, or nil if none is scheduled.
// It is derived, so changing or clearing rehab cannot leave a stale cache.
// An overdue Stage review stays overdue even while rehab is active.
func (p UserMaterialProgress) NextReviewAt() *time.Time {
	next := p.StageReviewAt
	if p.RehabActive && p.RehabReviewAt != nil &&
		(next == nil || p.RehabReviewAt.Before(*next)) {
		next = p.RehabReviewAt
	}
	if p.ExtraReviewAt != nil && (next == nil || p.ExtraReviewAt.Before(*next)) {
		next = p.ExtraReviewAt
	}
	if next == nil {
		return nil
	}

	// Do not expose a pointer through which callers could mutate the schedule.
	result := *next
	return &result
}

// ScheduleRehab starts or updates an insertion into the Stage schedule.
// The algorithm supplies the step and date; Stage remains the long-term state.
func (p *UserMaterialProgress) ScheduleRehab(step int, reviewAt time.Time) {
	p.RehabActive = true
	p.RehabStep = step
	p.RehabReviewAt = &reviewAt
	p.RehabConsecutiveCorrect = 0
}

// EndRehab clears rehab after successful completion or SKIP REHAB. It does not
// restart the Stage timer, change Stage, or reset answer counters. The caller
// records the corresponding training event and any answer statistics.
func (p *UserMaterialProgress) EndRehab() {
	p.RehabActive = false
	p.RehabReviewAt = nil
	p.RehabStep = 0
	p.RehabConsecutiveCorrect = 0
}

// CancelRecovery is used by an ordinary Stage review, which takes precedence
// over every due recovery action, even if recovery has been overdue longer.
func (p *UserMaterialProgress) CancelRecovery() {
	p.EndRehab()
	p.ExtraReviewAt = nil
}

func (p UserMaterialProgress) EffectiveDifficulty(base materialmodel.Difficulty) materialmodel.Difficulty {
	if p.DifficultyOverride != nil {
		return *p.DifficultyOverride
	}
	return base
}
