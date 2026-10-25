package postgres

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"time"
)

type presetRepository struct{ tx *runtimeTx }

func (t *runtimeTx) MockPresets() repository.MockPresetRepository { return &presetRepository{t} }
func (r *presetRepository) List() ([]model.MockPreset, error) {
	out := []model.MockPreset{}
	err := r.tx.db.Where("user_id = ?", r.tx.user).Order("created_at DESC, id").Find(&out).Error
	return out, translate(err)
}
func (r *presetRepository) Get(id uuid.UUID) (model.MockPreset, error) {
	var out model.MockPreset
	err := r.tx.db.Where("user_id = ? AND id = ?", r.tx.user, id).Take(&out).Error
	return out, translate(err)
}
func (r *presetRepository) Create(p model.MockPreset) error {
	if p.UserID != r.tx.user {
		return repository.ErrNotFound
	}
	return translate(r.tx.db.Create(&p).Error)
}
func (r *presetRepository) Update(p model.MockPreset, version int) error {
	if p.UserID != r.tx.user {
		return repository.ErrNotFound
	}
	result := r.tx.db.Model(&model.MockPreset{}).Where("user_id = ? AND id = ? AND version = ?", r.tx.user, p.ID, version).
		Select("Name", "FolderIDs", "Settings", "Version", "UpdatedAt").Updates(&p)
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}
func (r *presetRepository) Delete(id uuid.UUID) error {
	result := r.tx.db.Where("user_id = ? AND id = ?", r.tx.user, id).Delete(&model.MockPreset{})
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected != 1 {
		return repository.ErrNotFound
	}
	return nil
}
func (r *presetRepository) Used(id uuid.UUID, at time.Time) error {
	return translate(r.tx.db.Model(&model.MockPreset{}).Where("user_id = ? AND id = ?", r.tx.user, id).Update("last_used_at", at).Error)
}
