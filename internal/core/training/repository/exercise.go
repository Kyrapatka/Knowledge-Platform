package repository

import (
	"context"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
)

// ExerciseTx is the CRUD boundary; picking exercises remains part of session commands.
type ExerciseTx interface {
	Exercises(uuid.UUID, int, int) ([]model.FormulaExercise, error)
	Exercise(uuid.UUID, uuid.UUID) (model.FormulaExercise, error)
	CreateExercise(model.FormulaExercise) error
	UpdateExercise(model.FormulaExercise, int) error
	DeleteExercise(uuid.UUID, uuid.UUID, int) error
}
type ExerciseStore interface {
	TransactExercises(context.Context, uuid.UUID, func(ExerciseTx) error) error
}
type exerciseStore struct{ runtime RuntimeStore }

// ExercisesIn shares the runtime's ownership scope, user lock and transaction.
func ExercisesIn(runtime RuntimeStore) ExerciseStore { return exerciseStore{runtime} }
func (s exerciseStore) TransactExercises(ctx context.Context, user uuid.UUID, fn func(ExerciseTx) error) error {
	return s.runtime.Transact(ctx, user, func(tx Tx) error { return fn(tx) })
}
