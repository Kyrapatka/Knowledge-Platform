package interview

import (
	"context"
	"github.com/google/uuid"
	"time"
)

// Transact serializes account mutations. Every operation shares the user lock and commit.
type Repository interface {
	Transact(context.Context, uuid.UUID, func(Transaction) error) error
}
type Transaction interface {
	CopyContent(source, target uuid.UUID, allowDuplicate bool) (CopyResult, error)
	FolderTemplate(uuid.UUID) (string, error)
	MaterialValues(uuid.UUID, uuid.UUID) (map[string]*string, error)
	Profile(uuid.UUID) (Profile, error)
	PutProfile(Profile) error
	Catalog() (Catalog, error)
	PutConcept(Concept) error
	SeedProfile(string) (Profile, error)
	SeedMaterial(uuid.UUID) (seedMaterial, bool, error)
	PutSeedMaterial(uuid.UUID, uuid.UUID, map[string]*string, map[string]string, time.Time, bool) error
	SyncBank(SeedBank) error
	SeedFolder(SeedDomain) (uuid.UUID, error)
	RecordImport(ImportResult, SeedBank, []string, time.Time) error
}
