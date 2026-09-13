package model

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/google/uuid"
)

type UndoSnapshot struct {
	GraphStateBefore      *graph.State
	GraphSelectionEventID *uuid.UUID
	GraphProbe            bool
	EventID               uuid.UUID
	PlanID                uuid.UUID
	SessionID             uuid.UUID
	MaterialID            uuid.UUID
	ExpectedVersion       int
	PlanVersion           int
	Before                UserMaterialProgress
	Items                 []SessionItem
}
