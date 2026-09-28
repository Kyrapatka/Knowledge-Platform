package repository

import (
	"context"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
)

// ProgressViewTx cannot create progress or execute a training command.
type ProgressViewTx interface {
	Session(uuid.UUID) (model.TrainingSession, error)
	Plan(uuid.UUID) (model.TrainingPlan, error)
	Material(uuid.UUID) (material.Material, error)
	GetProgress(context.Context, ProgressKey) (model.UserMaterialProgress, error)
}
type ProgressViewStore interface {
	ReadProgress(context.Context, uuid.UUID, func(ProgressViewTx) error) error
}
type progressViewStore struct{ runtime RuntimeStore }
type progressViewTx struct{ Tx }

func (t progressViewTx) GetProgress(ctx context.Context, key ProgressKey) (model.UserMaterialProgress, error) {
	return t.Progress().Get(ctx, key)
}
func ProgressViewsIn(runtime RuntimeStore) ProgressViewStore { return progressViewStore{runtime} }
func (s progressViewStore) ReadProgress(ctx context.Context, user uuid.UUID, fn func(ProgressViewTx) error) error {
	return s.runtime.Transact(ctx, user, func(tx Tx) error { return fn(progressViewTx{tx}) })
}
