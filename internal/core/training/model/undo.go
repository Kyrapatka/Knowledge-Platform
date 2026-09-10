package model

import "github.com/google/uuid"

type UndoSnapshot struct {
	EventID         uuid.UUID
	PlanID          uuid.UUID
	SessionID       uuid.UUID
	MaterialID      uuid.UUID
	ExpectedVersion int
	PlanVersion     int
	Before          UserMaterialProgress
	Items           []SessionItem
}
