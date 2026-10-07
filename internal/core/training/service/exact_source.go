package service

import (
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
)

func containsMaterial(ids []uuid.UUID, id uuid.UUID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// Nil preserves existing folder/topic selection. An explicit empty set must not
// silently broaden the source to the entire folder.
func validateExactSource(tx repository.Tx, source model.SessionSource) error {
	if source.MaterialIDs == nil {
		return nil
	}
	if len(source.MaterialIDs) == 0 || len(source.MaterialIDs) > 1000 {
		return fmt.Errorf("%w: select between 1 and 1000 materials", ErrInvalid)
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range source.MaterialIDs {
		if id == uuid.Nil || seen[id] {
			return fmt.Errorf("%w: duplicate or invalid material selection", ErrInvalid)
		}
		seen[id] = true
	}
	return tx.CheckMaterialSelection(source.FolderID, source.MaterialIDs)
}
