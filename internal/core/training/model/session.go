package model

import (
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/google/uuid"
	"time"
)

type CardField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// A presentation snapshots content and difficulty so edits cannot change the
// meaning of an already displayed question. Answers identify this exact show.
type Presentation struct {
	InterviewGraph          *InterviewGraphPresentation `json:"interview_graph,omitempty"`
	Direction               string                      `json:"direction,omitempty"`
	Example                 string                      `json:"example,omitempty"`
	ForeignWord             string                      `json:"foreign_word,omitempty"`
	ExerciseID              *uuid.UUID                  `json:"exercise_id,omitempty"`
	ExerciseVersion         int                         `json:"exercise_version,omitempty"`
	PracticeMode            PracticeMode                `json:"practice_mode,omitempty"`
	FinalReview             bool                        `json:"final_review"`
	ID                      uuid.UUID                   `json:"id"`
	MaterialID              uuid.UUID                   `json:"material_id"`
	FolderID                uuid.UUID                   `json:"folder_id"`
	Kind                    ReviewKind                  `json:"kind"`
	ProgressVersion         int                         `json:"progress_version"`
	Stage                   int                         `json:"stage"`
	ConsecutiveCorrect      int                         `json:"consecutive_correct"`
	RehabConsecutiveCorrect int                         `json:"rehab_consecutive_correct"`
	RequiredCorrect         int                         `json:"required_correct"`
	Difficulty              material.Difficulty         `json:"difficulty"`
	Question                []CardField                 `json:"question"`
	Answer                  []CardField                 `json:"answer"`
	CreatedAt               time.Time                   `json:"created_at"`
}

type SessionItem struct {
	SessionID    uuid.UUID
	MaterialID   uuid.UUID
	Position     int64
	State        string
	Presentation *Presentation
}

type TrainingEvent struct {
	GraphSelectionEventID *uuid.UUID    `json:"graph_selection_event_id,omitempty"`
	ReviewCredit          bool          `json:"review_credit"`
	EventMode             string        `json:"event_mode"`
	Direction             string        `json:"direction,omitempty"`
	ExerciseID            *uuid.UUID    `json:"exercise_id,omitempty"`
	PracticeMode          *PracticeMode `json:"practice_mode,omitempty"`
	ID                    uuid.UUID     `json:"id"`
	CommandID             uuid.UUID     `json:"command_id"`
	UserID                uuid.UUID     `json:"user_id"`
	PlanID                uuid.UUID     `json:"plan_id"`
	SessionID             uuid.UUID     `json:"session_id"`
	MaterialID            uuid.UUID     `json:"material_id"`
	PresentationID        *uuid.UUID    `json:"presentation_id,omitempty"`
	Action                string        `json:"action"`
	Kind                  ReviewKind    `json:"kind"`
	AlgorithmKey          string        `json:"algorithm_key"`
	AlgorithmVersion      int           `json:"algorithm_version"`
	StageBefore           int           `json:"stage_before"`
	StageAfter            int           `json:"stage_after"`
	ProgressVersionBefore int           `json:"progress_version_before"`
	ProgressVersionAfter  int           `json:"progress_version_after"`
	CreatedAt             time.Time     `json:"created_at"`
}

type CommandReceipt struct {
	UserID      uuid.UUID
	CommandID   uuid.UUID
	RequestHash string
	Response    []byte
}

type SessionSummary struct {
	ScheduledReviews  int `json:"scheduled_reviews"`
	InterviewProbes   int `json:"interview_probes"`
	Correct           int `json:"correct"`
	Wrong             int `json:"wrong"`
	Advance           int `json:"advance"`
	Rollback          int `json:"rollback"`
	SkipRehab         int `json:"skip_rehab"`
	MaterialsReviewed int `json:"materials_reviewed"`
	StagePromotions   int `json:"stage_promotions"`
}

type SessionView struct {
	Graph       *InterviewGraphView `json:"graph,omitempty"`
	UndoActions []uuid.UUID         `json:"undo_actions,omitempty"`
	Session     TrainingSession     `json:"session"`
	Current     *Presentation       `json:"current"`
	PoolSize    int                 `json:"pool_size"`
	Summary     SessionSummary      `json:"summary"`
}

type ActionResult struct {
	Event        TrainingEvent `json:"event"`
	NextReviewAt *time.Time    `json:"next_review_at"`
	Session      SessionView   `json:"session"`
}
