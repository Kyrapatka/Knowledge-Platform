package postgres

import (
	"time"

	"github.com/google/uuid"
)

type folderModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	OwnerID       uuid.UUID `gorm:"type:uuid;not null;index"`
	Title         string    `gorm:"not null"`
	Description   string
	TemplateKey   string `gorm:"not null"`
	Config        []byte `gorm:"type:jsonb;not null"`
	ConfigVersion int64  `gorm:"not null;default:1"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (folderModel) TableName() string {
	return "folders"
}
