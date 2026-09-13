package repository

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"time"
)

type InterviewGraphRepository interface {
	RootCandidates(model.TrainingPlan, model.TrainingSession, time.Time) ([]graph.Candidate, error)
	FollowUpCandidates(model.TrainingPlan, model.TrainingSession, time.Time) ([]graph.Candidate, error)
	Catalog() (graph.Catalog, error)
	State(uuid.UUID) (graph.State, error)
	SaveState(uuid.UUID, graph.State, int, time.Time) (graph.State, error)
	SaveSelection(model.GraphSelectionEvent) error
	Selection(uuid.UUID, uuid.UUID) (model.GraphSelectionEvent, error)
	LatestSelection(uuid.UUID) (*model.GraphSelectionEvent, error)
	UndoSelection(uuid.UUID, uuid.UUID, time.Time) error
	Statistics(uuid.UUID) (model.GraphStatistics, error)
}
