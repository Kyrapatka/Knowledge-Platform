package postgres

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type materialModel struct {
	DeletedAt gorm.DeletedAt
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	FolderID  uuid.UUID `gorm:"type:uuid;not null;index"`

	Values     []byte `gorm:"type:jsonb;not null"`
	Metadata   []byte `gorm:"type:jsonb;not null"`
	Difficulty string `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (materialModel) TableName() string {
	return "materials"
}
