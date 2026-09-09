package model

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type PlanChange struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"user_id"`
	PlanID    uuid.UUID       `json:"plan_id"`
	CommandID uuid.UUID       `json:"command_id"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}
