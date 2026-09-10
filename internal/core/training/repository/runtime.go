package repository

import (
	"context"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"time"
)

// Transact serializes training commands for the same user. Every method on Tx
// is ownership scoped and participates in the same database transaction.
type RuntimeStore interface {
	Transact(context.Context, uuid.UUID, func(Tx) error) error
}

type Tx interface {
	SaveUndo(model.UndoSnapshot) error
	UndoHistory() ([]model.UndoSnapshot, error)
	RestoreUndo(model.UndoSnapshot, int, time.Time) error
	LastEnglishDirection(uuid.UUID) (string, error)
	Folder(uuid.UUID) (folder.Folder, error)
	SaveDefaults(uuid.UUID, folderconfig.TrainingConfig, int64, time.Time) error
	HasPlanOverlap([]uuid.UUID, model.ProgressTrack) (bool, error)
	HasIncompatibleProgress([]uuid.UUID, model.ProgressTrack, string, int) (bool, error)
	CreatePlan(model.TrainingPlan) error
	UpdatePlan(model.TrainingPlan, int) error
	PlanProgress(model.TrainingPlan) ([]model.UserMaterialProgress, error)
	SavePlanChange(model.PlanChange) error
	PlanChanges(uuid.UUID, int, int) ([]model.PlanChange, error)
	Plan(uuid.UUID) (model.TrainingPlan, error)
	Plans(int, int) ([]model.TrainingPlan, error)
	CancelPlan(uuid.UUID, time.Time) error
	CompletePlanIfReady(model.TrainingPlan, time.Time) (bool, error)
	Session(uuid.UUID) (model.TrainingSession, error)
	ActiveSession(uuid.UUID) (model.TrainingSession, error)
	LatestSession(uuid.UUID) (model.TrainingSession, error)
	Availability(model.TrainingPlan, []model.SessionSource) (model.Availability, error)
	CombinedSummary([]uuid.UUID) (model.SessionSummary, error)
	CreateSession(model.TrainingSession) error
	FinishSession(uuid.UUID, model.Status, time.Time) error
	Items(uuid.UUID) ([]model.SessionItem, error)
	SaveItem(model.SessionItem) error
	Candidates(model.TrainingPlan, uuid.UUID, time.Time, int) ([]material.Material, error)
	Material(uuid.UUID) (material.Material, error)
	Exercises(uuid.UUID, int, int) ([]model.FormulaExercise, error)
	Exercise(uuid.UUID, uuid.UUID) (model.FormulaExercise, error)
	CreateExercise(model.FormulaExercise) error
	UpdateExercise(model.FormulaExercise, int) error
	DeleteExercise(uuid.UUID, uuid.UUID, int) error
	PickExercise(uuid.UUID, uuid.UUID) (model.FormulaExercise, error)
	Progress() ProgressRepository
	Receipt(uuid.UUID) (model.CommandReceipt, error)
	SaveReceipt(model.CommandReceipt) error
	SaveEvent(model.TrainingEvent) error
	Summary(uuid.UUID) (model.SessionSummary, error)
}
