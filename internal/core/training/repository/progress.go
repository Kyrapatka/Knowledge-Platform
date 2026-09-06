package repository

import (
	"context"
	"errors"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"time"
)

var ErrNotFound = errors.New("training progress not found")
var ErrConflict = errors.New("training progress changed or already exists")

type ProgressKey struct {
	UserID     uuid.UUID
	MaterialID uuid.UUID
	Track      model.ProgressTrack
	PlanID     *uuid.UUID
}

type ProgressRepository interface {
	Create(context.Context, model.UserMaterialProgress) error
	Get(context.Context, ProgressKey) (model.UserMaterialProgress, error)
	Update(context.Context, model.UserMaterialProgress, int) (model.UserMaterialProgress, error)
	ListDue(context.Context, uuid.UUID, model.ProgressTrack, time.Time, int) ([]model.UserMaterialProgress, error)
}
