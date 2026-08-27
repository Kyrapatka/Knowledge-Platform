package handler

import (
	"time"

	"github.com/google/uuid"
)

type CreateMaterialRequest struct {
	Values   map[string]*string `json:"values"`
	Metadata map[string]*string `json:"metadata"`
}

type UpdateMaterialRequest struct {
	Values   map[string]*string `json:"values"`
	Metadata map[string]*string `json:"metadata"`
}

type MaterialResponse struct {
	ID       uuid.UUID `json:"id"`
	FolderID uuid.UUID `json:"folder_id"`

	Values   map[string]*string `json:"values"`
	Metadata map[string]*string `json:"metadata"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
