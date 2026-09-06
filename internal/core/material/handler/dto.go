package handler

import (
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"time"

	"github.com/google/uuid"
)

type CreateMaterialRequest struct {
	Difficulty *materialmodel.Difficulty `json:"difficulty"`
	Values     map[string]*string        `json:"values"`
	Metadata   map[string]*string        `json:"metadata"`
}

type UpdateMaterialRequest struct {
	Difficulty *materialmodel.Difficulty `json:"difficulty"`
	Values     map[string]*string        `json:"values"`
	Metadata   map[string]*string        `json:"metadata"`
}

type MaterialResponse struct {
	Difficulty materialmodel.Difficulty `json:"difficulty"`
	ID         uuid.UUID                `json:"id"`
	FolderID   uuid.UUID                `json:"folder_id"`

	Values   map[string]*string `json:"values"`
	Metadata map[string]*string `json:"metadata"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
