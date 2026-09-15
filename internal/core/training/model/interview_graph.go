package model

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/google/uuid"
	"time"
)

type SelectionStrategy string

const (
	SelectionRandom           SelectionStrategy = "random"
	SelectionInterviewGraphV1 SelectionStrategy = "interview_graph_v1"
)

type InterviewGraphPresentation struct {
	SelectionEventID uuid.UUID `json:"selection_event_id"`
	RootIndex        int       `json:"root_index"`
	Depth            int       `json:"depth"`
	Probe            bool      `json:"probe"`
	ReviewCredit     bool      `json:"review_credit"`
	BankVerification bool      `json:"bank_verification"`
	AnswerIncomplete bool      `json:"answer_incomplete"`
}
type GraphSelectionEvent struct {
	ID               uuid.UUID       `json:"id"`
	SessionID        uuid.UUID       `json:"session_id"`
	FromMaterialID   *uuid.UUID      `json:"from_material_id,omitempty"`
	ToMaterialID     uuid.UUID       `json:"to_material_id"`
	RootIndex        int             `json:"root_index"`
	SelectionOrder   int             `json:"selection_order"`
	DepthBefore      int             `json:"depth_before"`
	DepthAfter       int             `json:"depth_after"`
	DetectedConcepts []graph.Match   `json:"detected_concepts" gorm:"serializer:json;type:jsonb"`
	Candidates       []graph.Score   `json:"candidates" gorm:"serializer:json;type:jsonb"`
	SelectionReason  string          `json:"selection_reason"`
	ReviewCredit     bool            `json:"review_credit"`
	RandomSeed       int64           `json:"random_seed"`
	RawAnswer        *string         `json:"-"`
	Snapshot         graph.Candidate `json:"selected_question" gorm:"serializer:json;type:jsonb"`
	CreatedAt        time.Time       `json:"created_at"`
}
type ConceptCoverage struct {
	Slug    string `json:"slug"`
	Asked   int    `json:"asked"`
	Correct int    `json:"correct"`
	Wrong   int    `json:"wrong"`
}
type GraphStatistics struct {
	ScheduledReviews       int               `json:"scheduled_reviews"`
	InterviewProbes        int               `json:"interview_probes"`
	Correct                int               `json:"correct"`
	Wrong                  int               `json:"wrong"`
	MaxDepth               int               `json:"max_depth"`
	AverageDepth           float64           `json:"average_depth"`
	CrossDomainTransitions int               `json:"cross_domain_transitions"`
	ConceptCoverage        []ConceptCoverage `json:"concept_coverage"`
}
type InterviewGraphView struct {
	State      graph.State          `json:"state"`
	Selection  *GraphSelectionEvent `json:"selection,omitempty"`
	Statistics GraphStatistics      `json:"statistics"`
}
