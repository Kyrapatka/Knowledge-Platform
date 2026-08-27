package repository

import (
	"context"

	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/google/uuid"
)

type Repository interface {
	Create(
		ctx context.Context,
		material materialmodel.Material,
	) error

	GetByID(
		ctx context.Context,
		id uuid.UUID,
	) (materialmodel.Material, error)

	ListByFolder(
		ctx context.Context,
		folderID uuid.UUID,
	) ([]materialmodel.Material, error)

	Update(
		ctx context.Context,
		material materialmodel.Material,
	) error

	Delete(
		ctx context.Context,
		id uuid.UUID,
	) error
}
