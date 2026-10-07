package postgres

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

func (t *runtimeTx) CheckMaterialSelection(folder uuid.UUID, ids []uuid.UUID) error {
	var count int64
	err := t.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Where("f.owner_id=? AND f.id=? AND f.deleted_at IS NULL AND m.deleted_at IS NULL AND m.id IN ?", t.user, folder, ids).Count(&count).Error
	if err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return repository.ErrNotFound
	}
	return nil
}
