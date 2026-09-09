package postgres

import (
	"encoding/json"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

func (t *runtimeTx) UpdatePlan(p model.TrainingPlan, version int) error {
	if p.UserID != t.user {
		return repository.ErrNotFound
	}
	config, err := json.Marshal(p.Config)
	if err != nil {
		return err
	}
	r := t.db.Table("training_plans").Where("id=? AND user_id=? AND version=? AND status='active'", p.ID, t.user, version).
		Updates(map[string]any{"algorithm_key": p.AlgorithmKey, "algorithm_version": p.AlgorithmVersion, "config": config, "version": version + 1, "updated_at": p.UpdatedAt})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}
func (t *runtimeTx) PlanProgress(p model.TrainingPlan) ([]model.UserMaterialProgress, error) {
	if p.UserID != t.user {
		return nil, repository.ErrNotFound
	}
	rows := make([]model.UserMaterialProgress, 0)
	q := t.db.Table("user_material_progress p").Select("p.*").Joins("JOIN materials m ON m.id=p.material_id").Joins("JOIN folders f ON f.id=m.folder_id").
		Where("p.user_id=? AND p.track=? AND f.owner_id=? AND m.folder_id IN ? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", t.user, p.Track, t.user, p.SourceFolderIDs)
	if p.Track == model.ProgressTrackCram {
		q = q.Where("p.plan_id=?", p.ID)
	} else {
		q = q.Where("p.plan_id IS NULL")
	}
	err := q.Order("p.material_id").Find(&rows).Error
	return rows, err
}
func (t *runtimeTx) SavePlanChange(change model.PlanChange) error {
	if change.UserID != t.user {
		return repository.ErrNotFound
	}
	return t.db.Table("training_plan_changes").Create(map[string]any{"id": change.ID, "user_id": change.UserID, "plan_id": change.PlanID, "command_id": change.CommandID, "details": []byte(change.Details), "created_at": change.CreatedAt}).Error
}
func (t *runtimeTx) PlanChanges(planID uuid.UUID, limit, offset int) ([]model.PlanChange, error) {
	if _, err := t.Plan(planID); err != nil {
		return nil, err
	}
	rows := make([]model.PlanChange, 0)
	err := t.db.Table("training_plan_changes").Where("plan_id=? AND user_id=?", planID, t.user).Order("created_at DESC,id").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, err
}
