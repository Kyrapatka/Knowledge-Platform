package postgres

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

func (t *runtimeTx) Exercises(materialID uuid.UUID, limit, offset int) ([]model.FormulaExercise, error) {
	if _, err := t.Material(materialID); err != nil {
		return nil, err
	}
	out := make([]model.FormulaExercise, 0)
	err := t.db.Table("formula_exercises").Where("material_id=?", materialID).Order("created_at,id").Limit(limit).Offset(offset).Find(&out).Error
	return out, err
}
func (t *runtimeTx) Exercise(materialID, id uuid.UUID) (model.FormulaExercise, error) {
	var out model.FormulaExercise
	if _, err := t.Material(materialID); err != nil {
		return out, err
	}
	err := t.db.Table("formula_exercises").Where("material_id=? AND id=?", materialID, id).Take(&out).Error
	return out, translate(err)
}
func (t *runtimeTx) CreateExercise(e model.FormulaExercise) error {
	if _, err := t.Material(e.MaterialID); err != nil {
		return err
	}
	return t.db.Table("formula_exercises").Create(&e).Error
}
func (t *runtimeTx) UpdateExercise(e model.FormulaExercise, version int) error {
	if _, err := t.Exercise(e.MaterialID, e.ID); err != nil {
		return err
	}
	r := t.db.Table("formula_exercises").Where("material_id=? AND id=? AND version=?", e.MaterialID, e.ID, version).
		Updates(map[string]any{"problem": e.Problem, "answer": e.Answer, "solution": e.Solution, "hint": e.Hint, "version": version + 1, "updated_at": e.UpdatedAt})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}
func (t *runtimeTx) DeleteExercise(materialID, id uuid.UUID, version int) error {
	if _, err := t.Exercise(materialID, id); err != nil {
		return err
	}
	r := t.db.Table("formula_exercises").Where("material_id=? AND id=? AND version=?", materialID, id, version).Delete(&model.FormulaExercise{})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}

// Prefer a different exercise from the last answered one for this material.
// Only the selected exercise is persisted, so resume never rerolls a task.
func (t *runtimeTx) PickExercise(materialID, planID uuid.UUID) (model.FormulaExercise, error) {
	var out model.FormulaExercise
	if _, err := t.Material(materialID); err != nil {
		return out, err
	}
	if _, err := t.Plan(planID); err != nil {
		return out, err
	}
	err := t.db.Raw(`SELECT e.* FROM formula_exercises e WHERE e.material_id=?
 ORDER BY COALESCE(e.id=(SELECT exercise_id FROM training_events
 WHERE user_id=? AND plan_id=? AND material_id=? AND undone_at IS NULL AND exercise_id IS NOT NULL AND action IN ('correct','wrong')
 ORDER BY created_at DESC,progress_version_after DESC LIMIT 1),false),random() LIMIT 1`, materialID, t.user, planID, materialID).Scan(&out).Error
	if err == nil && out.ID == uuid.Nil {
		err = repository.ErrNotFound
	}
	return out, err
}
