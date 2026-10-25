package repository

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"time"
)

type MockPresetRepository interface {
	List() ([]model.MockPreset, error)
	Get(uuid.UUID) (model.MockPreset, error)
	Create(model.MockPreset) error
	Update(model.MockPreset, int) error
	Delete(uuid.UUID) error
	Used(uuid.UUID, time.Time) error
}
