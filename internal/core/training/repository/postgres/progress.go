package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

const progressTable = "user_material_progress"

type ProgressRepository struct{ db *gorm.DB }

// Pass a transaction-bound DB when coordinating progress and events. A future
// answer service must save both in one transaction, not call Update in isolation.
func NewProgressRepository(db *gorm.DB) *ProgressRepository { return &ProgressRepository{db: db} }

var _ repository.ProgressRepository = (*ProgressRepository)(nil)

func (r *ProgressRepository) Create(ctx context.Context, p model.UserMaterialProgress) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Version != 1 {
		return fmt.Errorf("new progress must have version 1")
	}
	// The current MVP only trains the user's own materials.
	var count int64
	if err := r.db.WithContext(ctx).Table("materials m").Joins("JOIN folders f ON f.id = m.folder_id").
		Where("m.id = ? AND f.owner_id = ?", p.MaterialID, p.UserID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return repository.ErrNotFound
	}
	return translate(r.db.WithContext(ctx).Table(progressTable).Create(progressValues(p)).Error)
}

func (r *ProgressRepository) Get(ctx context.Context, key repository.ProgressKey) (model.UserMaterialProgress, error) {
	var p model.UserMaterialProgress
	err := scoped(r.db.WithContext(ctx).Table(progressTable), key).Take(&p).Error
	return p, translate(err)
}

func (r *ProgressRepository) Update(ctx context.Context, p model.UserMaterialProgress, expected int) (model.UserMaterialProgress, error) {
	if err := p.Validate(); err != nil {
		return p, err
	}
	if expected < 1 || p.Version != expected {
		return p, repository.ErrConflict
	}
	key := repository.ProgressKey{UserID: p.UserID, MaterialID: p.MaterialID, Track: p.Track, PlanID: p.PlanID}
	next := p
	next.Version++
	values := progressValues(next)
	for _, immutable := range []string{"user_id", "material_id", "track", "plan_id", "created_at"} {
		delete(values, immutable)
	}
	result := scoped(r.db.WithContext(ctx).Table(progressTable), key).Where("version = ?", expected).Updates(values)
	if result.Error != nil {
		return p, translate(result.Error)
	}
	if result.RowsAffected != 1 {
		return p, repository.ErrConflict
	}
	return next, nil
}

func (r *ProgressRepository) ListDue(ctx context.Context, userID uuid.UUID, track model.ProgressTrack, now time.Time, limit int) ([]model.UserMaterialProgress, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("due limit must be between 1 and 1000")
	}
	var rows []model.UserMaterialProgress
	err := r.db.WithContext(ctx).Table(progressTable).
		Where("user_id = ? AND track = ? AND next_review_at <= ?", userID, track, now).
		Order("next_review_at, material_id, plan_id NULLS FIRST").Limit(limit).Find(&rows).Error
	return rows, translate(err)
}

func scoped(db *gorm.DB, key repository.ProgressKey) *gorm.DB {
	db = db.Where("user_id = ? AND material_id = ? AND track = ?", key.UserID, key.MaterialID, key.Track)
	if key.PlanID == nil {
		return db.Where("plan_id IS NULL")
	}
	return db.Where("plan_id = ?", *key.PlanID)
}

func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return repository.ErrConflict
	}
	return err
}

func progressValues(p model.UserMaterialProgress) map[string]any {
	return map[string]any{
		"user_id": p.UserID, "material_id": p.MaterialID, "track": p.Track, "plan_id": p.PlanID,
		"algorithm_key": p.AlgorithmKey, "algorithm_version": p.AlgorithmVersion, "stage": p.Stage,
		"correct_count": p.CorrectCount, "wrong_count": p.WrongCount,
		"consecutive_correct": p.ConsecutiveCorrect, "consecutive_wrong": p.ConsecutiveWrong,
		"difficulty_override": p.DifficultyOverride, "learning_started_at": p.LearningStartedAt,
		"target_at": p.TargetAt, "completed_at": p.CompletedAt,
		"stage_last_review_at": p.StageLastReviewAt, "stage_review_at": p.StageReviewAt,
		"rehab_active": p.RehabActive, "rehab_step": p.RehabStep, "rehab_review_at": p.RehabReviewAt,
		"rehab_consecutive_correct": p.RehabConsecutiveCorrect, "extra_review_at": p.ExtraReviewAt,
		"version": p.Version, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	}
}
